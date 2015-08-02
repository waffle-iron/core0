package builtin

import (
    "github.com/Jumpscale/jsagent/agent/lib/pm"
    "time"
)

type Runable func (*pm.Cmd, pm.RunCfg) *pm.JobResult

type InternalProcess struct {
    runable Runable
    cmd *pm.Cmd
}

func InternalProcessFactory (runable Runable) pm.ProcessConstructor {
    constructor := func(cmd *pm.Cmd) pm.Process {
        return &InternalProcess{
            runable: runable,
            cmd: cmd,
        }
    }

    return constructor
}

func (ps *InternalProcess) Cmd() * pm.Cmd {
    return ps.cmd
}

func (ps *InternalProcess) Run(cfg pm.RunCfg) {
    defer func() {
        cfg.Signal <- 1
    }()

    starttime := time.Duration(time.Now().UnixNano()) / time.Millisecond // start time in msec
    result := ps.runable(ps.cmd, cfg)

    if result != nil {
        result.StartTime = int64(starttime)
        endtime := time.Duration(time.Now().UnixNano()) / time.Millisecond
        result.Time = int64(endtime - starttime)
        cfg.ResultHandler(result)
    }
}

func (ps *InternalProcess) Kill() {
    //you can't kill an internal process.
}

func (ps *InternalProcess) GetStats() *pm.ProcessStats {
    //can't provide values for the internal process, but
    //we have to return the correct data struct for interface completenss
    //also indication of running internal commands.
    return &pm.ProcessStats {
        Cmd: ps.cmd,
        CPU: 0,
        RSS: 0,
        VMS: 0,
        Swap: 0,
    }
}
