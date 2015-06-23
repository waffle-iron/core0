package pm

import (
    "io/ioutil"
    "os/exec"
    "time"
    "log"
    "fmt"
    "sync"
    "strconv"
    "github.com/Jumpscale/jsagent/agent/lib/utils"
    "github.com/shirou/gopsutil/process"
)

var RESULT_MESSAGE_LEVELS []int = []int{20, 21, 22, 23, 30}

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
    mid uint32
    midfile string
    midMux *sync.Mutex
    cmds chan *Cmd
    processes map[string]*Process
    meterHandlers []MeterHandler
    msgHandlers []MessageHandler
}


type runCfg struct {
    meterHandler MeterHandler
    msgHandler MessageHandler
}

func NewPM(midfile string) *PM {
    pm := &PM{
        cmds: make(chan *Cmd),
        midfile: midfile,
        mid: loadMid(midfile),
        midMux: &sync.Mutex{},
        processes: make(map[string]*Process),
        meterHandlers: make([]MeterHandler, 0, 3),
        msgHandlers: make([]MessageHandler, 0, 3),
    }
    return pm
}

func loadMid(midfile string) uint32 {
    content, err := ioutil.ReadFile(midfile)
    if err != nil {
        log.Println(err)
        return 0
    }
    v, err := strconv.ParseUint(string(content), 10, 32)
    if err != nil {
        log.Println(err)
        return 0
    }
    return uint32(v)
}

func saveMid(midfile string, mid uint32) {
    ioutil.WriteFile(midfile, []byte(fmt.Sprintf("%d", mid)), 0644)
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

func (pm *PM) getNextMsgID() uint32 {
    pm.midMux.Lock()
    defer pm.midMux.Unlock()
    pm.mid += 1
    saveMid(pm.midfile, pm.mid)
    return pm.mid
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
    if !utils.In(RESULT_MESSAGE_LEVELS, msg.level) &&
       !utils.In(msg.cmd.args.GetLogLevels(), msg.level) {
        return
    }

    //stamp msg.
    msg.epoch = time.Now().Unix()
    //add ID
    msg.id = pm.getNextMsgID()
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
