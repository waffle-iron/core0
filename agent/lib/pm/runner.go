package pm

import (
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/Jumpscale/agent2/agent/lib/pm/process"
	"github.com/Jumpscale/agent2/agent/lib/pm/stream"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	//"github.com/Jumpscale/agent2/agent/lib/pm/stream"
	"time"
)

// type RunnerCallbacks struct {
// 	ProcessManager *PM
// 	MeterHandler   MeterHandler
// 	MessageHandler stream.MessageHandler
// 	ResultHandler  ResultHandler
// 	//signal         chan int
// }

type Runner interface {
	Run()
}

type runnerImpl struct {
	manager *PM
	command *core.Cmd
	factory process.ProcessFactory
}

// func concatBuffer(buffer *list.List) string {
// 	var strbuf bytes.Buffer
// 	for l := buffer.Front(); l != nil; l = l.Next() {
// 		strbuf.WriteString(l.Value.(string))
// 		strbuf.WriteString("\n")
// 	}

// 	return strbuf.String()
// }

func NewRunner(manager *PM, command *core.Cmd, factory process.ProcessFactory) Runner {
	return &runnerImpl{
		manager: manager,
		command: command,
		factory: factory,
	}
}

func (runner *runnerImpl) run() {
	starttime := time.Duration(time.Now().UnixNano()) / time.Millisecond // start time in msec

	jobresult := core.NewBasicJobResult(runner.command)
	jobresult.State = core.StateError
	jobresult.StartTime = int64(starttime)

	defer func() {
		endtime := time.Duration(time.Now().UnixNano()) / time.Millisecond
		jobresult.Time = int64(endtime - starttime)
		runner.manager.resultCallback(runner.command, jobresult)
	}()

	process := runner.factory(runner.command)

	channel, err := process.Run()
	if err != nil {
		//this basically means process couldn't spawn
		//which indicates a problem with the command itself. So restart won't
		//do any good. It's better to terminate it immediately.
	}

	var result *stream.Message

loop:
	for {
		select {
		case message := <-channel:
			if utils.In(stream.ResultMessageLevels, message.Level) {
				result = message
			} else if message.Level == stream.LevelExitState {
				jobresult.State = message.Message
				break loop
			}
			//by default, all messages are forwarded to the manager for further processing.
			runner.manager.msgCallback(message)
		}
	}

	if result != nil {
		jobresult.Level = result.Level
		jobresult.Data = result.Message
	}

}

func (runner *runnerImpl) Run() {

}
