package pm

import (
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/Jumpscale/agent2/agent/lib/pm/process"
	"github.com/Jumpscale/agent2/agent/lib/pm/stream"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"log"
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

func NewRunner(manager *PM, command *core.Cmd, factory process.ProcessFactory) Runner {
	return &runnerImpl{
		manager: manager,
		command: command,
		factory: factory,
		kill:    make(chan int),
	}
}

func (runner *runnerImpl) timeout() <-chan time.Time {
	var timeout <-chan time.Time
	t := runner.command.Args.GetInt("max_time")
	if t > 0 {
		timeout = time.After(time.Duration(t) * time.Second)
	}
	return timeout
}

func (runner *runnerImpl) run() *core.JobResult {
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
	stdoutBuffer := stream.NewBuffer(StreamBufferSize)
	stderrBuffer := stream.NewBuffer(StreamBufferSize)

	timeout := runner.timeout()
loop:
	for {
		select {
		case <-runner.kill:
			process.Kill()
			jobresult.State = core.StateKilled
			break loop
		case <-timeout:
			process.Kill()
			jobresult.State = core.StateTimeout
			break loop
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
			runner.manager.msgCallback(runner.command, message)
		}
	}

	runner.process = nil

	//consume channel to the end to allow process to cleanup probabry
	for _ = range channel {
		//noop.
	}

	if result != nil {
		jobresult.Level = result.Level
		jobresult.Data = result.Message
	}

	jobresult.Streams = []string{
		stdoutBuffer.String(),
		stderrBuffer.String(),
	}

	return jobresult
}

func (runner *runnerImpl) Run() {
	runs := 0
	var result *core.JobResult
	defer func() {
		if result != nil {
			runner.manager.resultCallback(runner.command, result)
		}
	}()

loop:
	for {
		result = runner.run()
		if result.State == core.StateKilled {
			//we never restart a killed process.
			break
		}

		args := runner.command.Args

		//recurring
		maxRestart := args.GetInt("max_restart")
		restarting := false
		var restartIn time.Duration

		if result.State != core.StateSuccess && maxRestart > 0 {
			runs++
			if runs < maxRestart {
				log.Println("Restarting", runner.command, "due to upnormal exit status, trials", runs+1, "/", maxRestart)
				restarting = true
				restartIn = 1 * time.Second
			}
		}

		recurringPeriod := args.GetInt("recurring_period")
		if recurringPeriod > 0 {
			restarting = true
			restartIn = time.Duration(recurringPeriod) * time.Second
		}

		if restarting {
			log.Println("Recurring", runner.command, "in", restartIn)
			//TODO: respect the kill signal.
			select {
			case <-time.After(restartIn):
			case <-runner.kill:
				log.Println("Command ", runner.command, "Killed during scheduler sleep")
				result.State = core.StateKilled
				break loop
			}
		} else {
			break
		}
	}
}

func (runner *runnerImpl) Kill() {
	runner.kill <- 1
}

func (runner *runnerImpl) Process() process.Process {
	return runner.process
}
