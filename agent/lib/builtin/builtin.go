package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"time"
)

/*
Runable represents a runnable built in function that can be managed by the process manager.
*/
type Runable func(*pm.Cmd, pm.RunCfg) *pm.JobResult

/*
InternalProcess implements a Procss interface and represents an internal (go) process that can be managed by the process manager
*/
type InternalProcess struct {
	runable Runable
	cmd     *pm.Cmd
}

/*
InternalProcessFactory factory to build Runnable processes
*/
func InternalProcessFactory(runable Runable) pm.ProcessConstructor {
	constructor := func(cmd *pm.Cmd) pm.Process {
		return &InternalProcess{
			runable: runable,
			cmd:     cmd,
		}
	}

	return constructor
}

/*
Cmd returns the internal process command
*/
func (ps *InternalProcess) Cmd() *pm.Cmd {
	return ps.cmd
}

/*
Run runs the internal process
*/
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

/*
Kill kills internal process (not implemented)
*/
func (ps *InternalProcess) Kill() {
	//you can't kill an internal process.
}

/*
GetStats gets cpu, mem, etc.. consumption of internal process (not implemented)
*/
func (ps *InternalProcess) GetStats() *pm.ProcessStats {
	//can't provide values for the internal process, but
	//we have to return the correct data struct for interface completenss
	//also indication of running internal commands.
	return &pm.ProcessStats{
		Cmd:  ps.cmd,
		CPU:  0,
		RSS:  0,
		VMS:  0,
		Swap: 0,
	}
}
