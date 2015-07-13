package pm


import (
    "os/exec"
    "time"
    "fmt"
    "log"
    "github.com/Jumpscale/jsagent/agent/lib/utils"
    "github.com/shirou/gopsutil/process"
    "encoding/json"
)

const (
    L_STDOUT = 1  // stdout
    L_STDERR = 2  // stderr
    L_PUBLIC = 3  // message for endusers / public message
    L_OPERATOR = 4  // message for operator / internal message
    L_UNKNOWN = 5  // log msg (unstructured = level5, cat=unknown)
    L_STRUCTURED = 6  // log msg structured
    L_WARNING = 7  // warning message
    L_OPS_ERROR = 8  // ops error
    L_CRITICAL = 9  // critical error
    L_STATSD = 10  // statsd message(s) AVG
    L_RESULT_JSON = 20  // result message, json
    L_RESULT_YAML = 21  // result message, yaml
    L_RESULT_TOML = 22  // result message, toml
    L_RESULT_HRD = 23  // result message, hrd
    L_RESULT_JOB = 30  // job, json (full result of a job)

    S_SUCCESS = "SUCCESS"
    S_ERROR = "ERROR"
    S_TIMEOUT = "TIMEOUT"
    S_KILLED = "KILLED"
)

var RESULT_MESSAGE_LEVELS []int = []int{L_RESULT_JSON,
    L_RESULT_YAML, L_RESULT_TOML, L_RESULT_HRD, L_RESULT_JOB}

type Process interface{
    Run(RunCfg)
    Kill()
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

type Message struct {
    Id uint32
    Cmd *Cmd
    Level int
    Message string
    Epoch int64
}

func (msg *Message) MarshalJSON() ([]byte, error) {
    data := make(map[string]interface{})
    args := msg.Cmd.Args
    data["domain"] = args.GetString("domaing")
    data["name"] = args.GetString("name")
    data["epoch"] = msg.Epoch
    data["level"] = msg.Level
    data["id"] = msg.Id
    data["data"] = msg.Message

    return json.Marshal(data)
}

func (msg *Message) String() string {
    return fmt.Sprintf("%s|%d:%s", msg.Cmd, msg.Level, msg.Message)
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
func (ps *ExtProcess) Run(cfg RunCfg) {
    args := ps.cmd.Args
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

    starttime := time.Duration(time.Now().UnixNano()) / time.Millisecond // start time in msec

    err = cmd.Start()
    if err != nil {
        log.Println("Failed to start process", err)
        return
    }

    ps.pid = cmd.Process.Pid
    var result *Message = nil

    msgInterceptor := func (msg *Message) {
        if utils.In(RESULT_MESSAGE_LEVELS, msg.Level) {
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

    if ps.cmd.Data != "" {
        //write data to command stdin.
        _, err = stdin.Write([]byte(ps.cmd.Data))
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
        case <- time.After(30 * time.Second):
            //monitor.
            cfg.MeterHandler(ps.cmd, psProcess)
        }
    }

    endtime := time.Duration(time.Now().UnixNano()) / time.Millisecond

    if endtime - starttime < 300 * time.Millisecond {
        //if process lived for more than 5 min before it dies, reset the runs
        //this means that only the max_restart count will be reached if the
        //process kept failing under the 5 min limit.
        ps.runs = 0
    }

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
            go ps.Run(cfg)
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
            ps.Run(cfg)
        }()
    }

    var state string
    if success {
        state = S_SUCCESS
    } else if timedout {
        state = S_TIMEOUT
    } else if killed {
        state = S_KILLED
    } else {
        state = S_ERROR
    }

    jobresult := &JobResult{
        Id: ps.cmd.Id,
        Gid: ps.cmd.Gid,
        Nid: ps.cmd.Nid,
        Cmd: ps.cmd.Name,
        Args: ps.cmd.Args,
        State: state,
        StartTime: int64(starttime),
        Time: int64(endtime - starttime),
    }

    if result != nil {
        jobresult.Data = result.Message
        jobresult.Level = result.Level
    }

    if success && restarting && result == nil {
        //this is a recurring task. No need to flud
        //AC with success status.
        return
    }

    //delegating the result.
    cfg.ResultHandler(jobresult)
}


func (ps *ExtProcess) Kill() {
    ps.ctrl <- 1
}

