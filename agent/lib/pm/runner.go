package pm

import (
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/Jumpscale/agent2/agent/lib/pm/process"
	//"github.com/Jumpscale/agent2/agent/lib/pm/stream"
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

func (runner *runnerImpl) Run() {

}
