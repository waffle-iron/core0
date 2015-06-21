package pm

import (
    //"os"
    "os/exec"
    "time"
    "log"
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


type PM struct {
    cmds chan *Cmd
    processes map[string]*Process
}


func NewPM() *PM {
    pm := new(PM)
    pm.cmds = make(chan *Cmd)
    pm.processes = make(map[string]*Process)

    return pm
}


func (pm *PM) NewCmd(name string, id string, args Args, data string) error {
    cmd := &Cmd {
        id: id,
        name: name,
        args: args,
        data: data,
    }

    pm.cmds <- cmd
    return nil
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
            go process.run()
        }
    }()
}


func (ps *Process) run() {
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
    outConsumer := NewStreamConsumer(stdout, 1)
    outConsumer.Consume(func (msg Message){
        log.Println(msg)
    })

    errConsumer := NewStreamConsumer(stderr, 2)
    errConsumer.Consume(func (msg Message){
        log.Println("ERROR", msg)
    })

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
        statsInterval = 2 //TODO, use value from configurations.
    }

    var success bool

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
            log.Println("monitor process ...")
        }
    }

    //process exited.
    log.Println("Exit status: ", success)

    if !success && args.GetMaxRestart() > 0 {
        ps.runs += 1
        if ps.runs < args.GetMaxRestart() {
            log.Println("Restarting ...")
            go ps.run()
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
            ps.run()
        }()
    }
}
