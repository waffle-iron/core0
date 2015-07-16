package builtin

import (
    "github.com/Jumpscale/jsagent/agent/lib/pm"
)

type Runable func (*pm.Cmd, pm.RunCfg)

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

    ps.runable(ps.cmd, cfg)
}

func (ps *InternalProcess) Kill (){
    //you can't kill an internal process.
}
