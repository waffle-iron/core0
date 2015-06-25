package pm


import (
    "os/exec"
    "time"
    "log"
    "github.com/Jumpscale/jsagent/agent/lib/utils"
    "github.com/shirou/gopsutil/process"
)

type Process interface{
    run(RunCfg)
    kill()
}

type RunCfg struct {
    ProcessManager *PM
    MeterHandler MeterHandler
    MessageHandler MessageHandler
    ResultHandler ResultHandler
    Signal chan int
}

type JobResult struct {
    Id string `json:"id"`
    Gid int `json:"gid"`
    Nid int `json:"nid"`
    Cmd string `json:"cmd"`
    Args Args `json:"args"`
    Data string `json:"data"`
    Level int `json:"level"`
    State string `json:"state"`
    StartTime int64 `json:"starttime"`
    Time int64 `json:"time"`
}

type ExtProcess struct {
    cmd *Cmd
    ctrl chan int
    pid int
    runs int
}

func NewExtProcess(cmd *Cmd) Process {
    return &ExtProcess{
        cmd: cmd,
        ctrl: make(chan int),
    }
}


//Start process, feed data over the process stdin, and start
//consuming both stdout, and stderr.
//All messages from the subprocesses are
func (ps *ExtProcess) run(cfg RunCfg) {
    args := ps.cmd.args
    cmd := exec.Command(args.GetString("name"),
                        args.GetStringArray("args")...)
    cmd.Dir = args.GetString("working_dir")

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        log.Println("Failed to open process stdout", err)
    }

    stderr, err := cmd.StderrPipe()
    if err != nil {
        log.Println("Failed to open process stderr", err)
    }

    stdin, err := cmd.StdinPipe()
    if err != nil {
        log.Println("Failed to open process stdin", err)
    }

    starttime := time.Now().Unix()

    err = cmd.Start()
    if err != nil {
        log.Println("Failed to start process", err)
        return
    }

    var result *Message = nil

    msgInterceptor := func (msg *Message) {
        if utils.In(RESULT_MESSAGE_LEVELS, msg.level) {
            //process result message.
            result = msg
        }

        cfg.MessageHandler(msg)
    }

    // start consuming outputs.
    outConsumer := NewStreamConsumer(ps.cmd, stdout, 1)
    outConsumer.Consume(msgInterceptor)

    errConsumer := NewStreamConsumer(ps.cmd, stderr, 2)
    errConsumer.Consume(msgInterceptor)

    if ps.cmd.data != "" {
        //write data to command stdin.
        _, err = stdin.Write([]byte(ps.cmd.data))
        if err != nil {
            log.Println("Failed to write to process stdin", err)
        }
    }

    stdin.Close()

    psexit := make(chan bool)


    go func() {
        //make sure all outputs are closed before waiting for the process
        //to exit.
        <- outConsumer.Signal
        <- errConsumer.Signal

        err := cmd.Wait()
        if err != nil {
            log.Println(err)
        }
        psexit <- cmd.ProcessState.Success()
    }()

    var timeout <-chan time.Time

    if args.GetInt("max_time") > 0 {
        timeout = time.After(time.Duration(args.GetInt("max_time")) * time.Second)
    }

    statsInterval := args.GetInt("stats_interval")
    if statsInterval == 0 {
        statsInterval = 30 //TODO, use value from configurations.
    }

    var success bool
    var timedout bool
    var killed bool

    psProcess, _ := process.NewProcess(int32(cmd.Process.Pid))

    loop:
    for {
        select {
        case success = <- psexit:
            //handle process exit
            log.Println("process exited normally")
            break loop
        case <- timeout:
            //process timed out.
            log.Println("process timed out")
            cmd.Process.Kill()
            success = false
            timedout = true
            break loop
        case s := <- ps.ctrl:
            if s == 1 {
                //kill signal
                log.Println("killing process")
                cmd.Process.Kill()
                success = false
                killed = true
                ps.runs = 0
                break loop
            }
        case <- time.After(time.Duration(statsInterval) * time.Second):
            //monitor.
            cfg.MeterHandler(ps.cmd, psProcess)
        }
    }

    endtime := time.Now().Unix()
    //process exited.
    log.Println("Exit status: ", success)
    restarting := false
    defer func() {
        if !restarting {
            close(ps.ctrl)
            cfg.Signal <- 1 // forces the PM to clean up
        }
    }()

    if !success && args.GetInt("max_restart") > 0 {
        ps.runs += 1
        if ps.runs < args.GetInt("max_restart") {
            log.Println("Restarting ...")
            restarting = true
            go ps.run(cfg)
        } else {
            log.Println("Not restarting")
        }
    }

    //recurring
    if args.GetInt("recurring_period") > 0 {
        restarting = true
        go func() {
            time.Sleep(time.Duration(args.GetInt("recurring_period")) * time.Second)
            ps.runs = 0
            log.Println("Recurring ...")
            ps.run(cfg)
        }()
    }

    var state string
    if success {
        state = "SUCCESS"
    } else if timedout {
        state = "TIMEOUT"
    } else if killed {
        state = "KILLED"
    } else {
        state = "ERROR"
    }

    jobresult := &JobResult{
        Id: ps.cmd.id,
        Cmd: ps.cmd.name,
        Args: ps.cmd.args,
        State: state,
        StartTime: starttime,
        Time: endtime - starttime,
    }

    if result != nil {
        jobresult.Data = result.message
        jobresult.Level = result.level
    }

    if success && restarting && result == nil {
        //this is a recurring task. No need to flud
        //AC with success status.
        return
    }

    //delegating the result.
    cfg.ResultHandler(jobresult)
}


func (ps *ExtProcess) kill() {
    ps.ctrl <- 1
}
