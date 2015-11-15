package pm

import (
	"bytes"
	"container/list"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/Jumpscale/agent2/agent/lib/pm/process"
	"github.com/Jumpscale/agent2/agent/lib/pm/stream"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"time"
)

const (
	StreamBufferSize = 1000
)

type Runner interface {
	Run()
	Kill()
	Process() process.Process
}

type runnerImpl struct {
	manager *PM
	command *core.Cmd
	factory process.ProcessFactory
	kill    chan int

	process process.Process
}

type limitedBuffer struct {
	size   int
	buffer *list.List
}

func newLimitedBuffer(size int) *limitedBuffer {
	return &limitedBuffer{
		size:   size,
		buffer: list.New(),
	}
}

func (buffer *limitedBuffer) String() string {
	var strbuf bytes.Buffer
	for l := buffer.buffer.Front(); l != nil; l = l.Next() {
		strbuf.WriteString(l.Value.(string))
		strbuf.WriteString("\n")
	}

	return strbuf.String()
}

func (buffer *limitedBuffer) Append(line string) {
	list := buffer.buffer
	list.PushBack(line)
	if list.Len() > buffer.size {
		list.Remove(list.Front())
	}
}

func NewRunner(manager *PM, command *core.Cmd, factory process.ProcessFactory) Runner {
	return &runnerImpl{
		manager: manager,
		command: command,
		factory: factory,
		kill:    make(chan int),
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

	runner.process = process

	channel, err := process.Run()
	if err != nil {
		//this basically means process couldn't spawn
		//which indicates a problem with the command itself. So restart won't
		//do any good. It's better to terminate it immediately.
	}

	var result *stream.Message
	stdoutBuffer := newLimitedBuffer(StreamBufferSize)
	stderrBuffer := newLimitedBuffer(StreamBufferSize)

loop:
	for {
		select {
		case <-runner.kill:
			process.Kill()
			jobresult.State = core.StateKilled
		case message := <-channel:
			if utils.In(stream.ResultMessageLevels, message.Level) {
				result = message
			} else if message.Level == stream.LevelExitState {
				jobresult.State = message.Message
				break loop
			}

			if message.Level == stream.LevelStdout {
				stdoutBuffer.Append(message.Message)
			} else if message.Level == stream.LevelStderr {
				stderrBuffer.Append(message.Message)
			}

			//by default, all messages are forwarded to the manager for further processing.
			runner.manager.msgCallback(message)
		}
	}

	runner.process = nil

	if result != nil {
		jobresult.Level = result.Level
		jobresult.Data = result.Message
	}

	jobresult.Streams = []string{
		stdoutBuffer.String(),
		stderrBuffer.String(),
	}

	runner.manager.resultCallback(runner.command, jobresult)
}

func (runner *runnerImpl) Run() {
	runner.run()
}

func (runner *runnerImpl) Kill() {
	runner.kill <- 1
}

func (runner *runnerImpl) Process() process.Process {
	return runner.process
}
