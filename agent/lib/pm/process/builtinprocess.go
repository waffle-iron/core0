package process

import (
	"encoding/json"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/Jumpscale/agent2/agent/lib/pm/stream"
)

/*
Runable represents a runnable built in function that can be managed by the process manager.
*/
type Runnable func(*core.Cmd) (interface{}, error)

/*
internalProcess implements a Procss interface and represents an internal (go) process that can be managed by the process manager
*/
type internalProcess struct {
	runnable Runnable
	cmd      *core.Cmd
}

func NewInternalProcess(cmd *core.Cmd, runnable Runnable) Process {
	return &internalProcess{
		runnable: runnable,
		cmd:      cmd,
	}
}

/*
internalProcessFactory factory to build Runnable processes
*/
func NewInternalProcessFactory(runnable Runnable) ProcessFactory {
	factory := func(cmd *core.Cmd) Process {
		return NewInternalProcess(cmd, runnable)
	}

	return factory
}

/*
Cmd returns the internal process command
*/
func (process *internalProcess) Cmd() *core.Cmd {
	return process.cmd
}

/*
Run runs the internal process
*/
func (process *internalProcess) Run() (<-chan *stream.Message, error) {

	channel := make(chan *stream.Message)

	go func(channel chan *stream.Message) {
		defer close(channel)
		value, err := process.runnable(process.cmd)
		msg := stream.Message{
			Level: stream.LevelResultJSON,
		}

		if err != nil {
			m, _ := json.Marshal(err)
			msg.Message = string(m)
		} else {
			m, _ := json.Marshal(value)
			msg.Message = string(m)
		}

		channel <- &msg
		if err != nil {
			channel <- stream.MessageExitError
		} else {
			channel <- stream.MessageExitSuccess
		}

	}(channel)

	return channel, nil
}

/*
Kill kills internal process (not implemented)
*/
func (process *internalProcess) Kill() {
	//you can't kill an internal process.
}

/*
GetStats gets cpu, mem, etc.. consumption of internal process (not implemented)
*/
func (process *internalProcess) GetStats() *ProcessStats {
	//can't provide values for the internal process, but
	//we have to return the correct data struct for interface completenss
	//also indication of running internal commands.
	return &ProcessStats{
		Cmd:  process.cmd,
		CPU:  0,
		RSS:  0,
		VMS:  0,
		Swap: 0,
	}
}
