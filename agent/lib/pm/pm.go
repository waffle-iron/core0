package pm

import (
    //"os"
    "os/exec"
    "time"
    "log"
    "github.com/shirou/gopsutil/process"
)


type Cmd struct {
    name string
    id string
    args Args
    data string
}


type Process struct {
    cmd *Cmd
    pid int
    runs int
}

type MeterHandler func (cmd *Cmd, p *process.Process)
type MessageHandler func (msg *Message)

type PM struct {
    cmds chan *Cmd
    processes map[string]*Process
    meterHandlers []MeterHandler
    msgHandlers []MessageHandler
}


type runCfg struct {
    meterHandler MeterHandler
    msgHandler MessageHandler
}

func NewPM() *PM {
    pm := new(PM)
    pm.cmds = make(chan *Cmd)
    pm.processes = make(map[string]*Process)
    pm.meterHandlers = make([]MeterHandler, 0, 3)
    pm.msgHandlers = make([]MessageHandler, 0, 3)
    return pm
}


func (pm *PM) NewCmd(name string, id string, args Args, data string) {
    cmd := &Cmd {
        id: id,
        name: name,
        args: args,
        data: data,
    }

    pm.cmds <- cmd
}

func (pm *PM) AddMeterHandler(handler MeterHandler) {
    pm.meterHandlers = append(pm.meterHandlers, handler)
}

func (pm *PM) AddMessageHandler(handler MessageHandler) {
    pm.msgHandlers = append(pm.msgHandlers, handler)
}

func (pm *PM) Run() {
    //process and start all commands according to args.
    go func() {
        for {
            cmd := <- pm.cmds
            process := &Process{
                cmd: cmd,
            }

            pm.processes[cmd.id] = process // do we really need this ?
            go process.run(runCfg{
                meterHandler: pm.meterCallback,
                msgHandler: pm.msgCallback,
            })
        }
    }()
}

func (pm *PM) meterCallback(cmd *Cmd, ps *process.Process) {
    for _, handler := range pm.meterHandlers {
        handler(cmd, ps)
    }
}

func (pm *PM) msgCallback(msg *Message) {
    for _, handler := range pm.msgHandlers {
        handler(msg)
    }
}

//Start process, feed data over the process stdin, and start
//consuming both stdout, and stderr.
//All messages from the subprocesses are
func (ps *Process) run(cfg runCfg) {
    args := ps.cmd.args
    cmd := exec.Command(args.GetName(),
                        args.GetCmdArgs()...)
    cmd.Dir = args.GetWorkingDir()

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

    err = cmd.Start()
    if err != nil {
        log.Println("Failed to start process", err)
        return
    }

    // start consuming outputs.
    outConsumer := NewStreamConsumer(ps.cmd, stdout, 1)
    outConsumer.Consume(cfg.msgHandler)

    errConsumer := NewStreamConsumer(ps.cmd, stderr, 2)
    errConsumer.Consume(cfg.msgHandler)

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
        <- outConsumer.Signal
        <- errConsumer.Signal
        err := cmd.Wait()
        if err != nil {
            log.Println(err)
        }
        psexit <- cmd.ProcessState.Success()
    }()

    var timeout <-chan time.Time

    if args.GetMaxTime() > 0 {
        timeout = time.After(time.Duration(args.GetMaxTime()) * time.Second)
    }

    statsInterval := args.GetStatsInterval()
    if statsInterval == 0 {
        statsInterval = 30 //TODO, use value from configurations.
    }

    var success bool

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
            break loop
        case <- time.After(time.Duration(statsInterval) * time.Second):
            //monitor.
            cfg.meterHandler(ps.cmd, psProcess)
        }
    }

    //process exited.
    log.Println("Exit status: ", success)

    if !success && args.GetMaxRestart() > 0 {
        ps.runs += 1
        if ps.runs < args.GetMaxRestart() {
            log.Println("Restarting ...")
            go ps.run(cfg)
        } else {
            log.Println("Not restarting")
        }
    }

    //recurring
    if success && args.GetRecurringPeriod() > 0 {
        go func() {
            time.Sleep(time.Duration(args.GetRecurringPeriod()) * time.Second)
            ps.runs = 0
            log.Println("Recurring ...")
            ps.run(cfg)
        }()
    }
}
