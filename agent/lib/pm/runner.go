package pm

import (
	"fmt"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/pm/process"
	"github.com/g8os/core/agent/lib/pm/stream"
	"github.com/g8os/core/agent/lib/stats"
	"github.com/g8os/core/agent/lib/utils"
	"strings"
	"sync"
	"time"
)

const (
	StreamBufferSize = 1000

	meterPeriod = 30 * time.Second
)

type RunnerHook func(bool)

type WaitHook interface {
	Hook(ok bool)
	Wait() bool
}

type waitHook struct {
	wg *sync.WaitGroup
	o  sync.Once
	ok bool
}

func NewWaitHook() WaitHook {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return &waitHook{
		wg: wg,
	}
}

func (w *waitHook) Hook(ok bool) {
	w.o.Do(func() {
		w.ok = ok
		w.wg.Done()
	})
}

func (w *waitHook) Wait() bool {
	w.wg.Wait()
	return w.ok
}

type Runner interface {
	Command() *core.Cmd
	Run()
	Kill()
	Process() process.Process
	Wait() *core.JobResult
}

type runnerImpl struct {
	manager *PM
	command *core.Cmd
	factory process.ProcessFactory
	kill    chan int

	process process.Process
	statsd  *stats.Statsd

	hooks       []RunnerHook
	hooksOnExit bool
	hookOnce    sync.Once

	waitOnce sync.Once
	result   *core.JobResult
	wg       sync.WaitGroup
}

/*
NewRunner creates a new runner object that is bind to this PM instance.

:manager: Bind this runner to this PM instance
:command: Command to run
:factory: Process factory associated with command type.
:hooks: Optionals hooks that are called if the process is considered RUNNING successfully.
        The process is considered running, if it ran with no errors for 2 seconds, or exited before the 2 seconds passes
        with SUCCESS exit code.
*/
func NewRunner(manager *PM, command *core.Cmd, factory process.ProcessFactory, hooksOnExit bool, hooks ...RunnerHook) Runner {
	statsInterval := command.Args.GetInt("stats_interval")

	if statsInterval == 0 {
		statsInterval = 30
	}

	prefix := fmt.Sprintf("%d.%d.%s.%s.%s", command.Gid, command.Nid, command.Name,
		command.Args.GetStringDefault("domain", "unknown"), command.Args.GetStringDefault("name", "unknown"))

	runner := &runnerImpl{
		manager:     manager,
		command:     command,
		factory:     factory,
		kill:        make(chan int),
		hooks:       hooks,
		hooksOnExit: hooksOnExit,

		statsd: stats.NewStatsd(
			prefix,
			time.Duration(statsInterval)*time.Millisecond,
			manager.statsFlushCallback),
	}

	runner.wg.Add(1)
	return runner
}

func (runner *runnerImpl) Command() *core.Cmd {
	return runner.command
}

func (runner *runnerImpl) timeout() <-chan time.Time {
	var timeout <-chan time.Time
	t := runner.command.Args.GetInt("max_time")
	if t > 0 {
		timeout = time.After(time.Duration(t) * time.Second)
	}
	return timeout
}

func (runner *runnerImpl) meter() {
	process := runner.process
	if process == nil {
		return
	}

	stats := process.GetStats()
	//feed statsd
	statsd := runner.statsd
	statsd.Gauage("_cpu_", fmt.Sprintf("%f", stats.CPU))
	statsd.Gauage("_rss_", fmt.Sprintf("%d", stats.RSS))
	statsd.Gauage("_vms_", fmt.Sprintf("%d", stats.VMS))
	statsd.Gauage("_swap_", fmt.Sprintf("%d", stats.Swap))
}

func (runner *runnerImpl) run() *core.JobResult {
	starttime := time.Duration(time.Now().UnixNano()) / time.Millisecond // start time in msec

	jobresult := core.NewBasicJobResult(runner.command)
	jobresult.State = core.StateError
	jobresult.StartTime = int64(starttime)

	defer func() {
		endtime := time.Duration(time.Now().UnixNano()) / time.Millisecond
		jobresult.Time = int64(endtime - starttime)
	}()

	process := runner.factory(runner.manager, runner.command)

	runner.process = process

	channel, err := process.Run()
	if err != nil {
		//this basically means process couldn't spawn
		//which indicates a problem with the command itself. So restart won't
		//do any good. It's better to terminate it immediately.
		jobresult.Data = err.Error()
		return jobresult
	}

	var result *stream.Message
	var critical string

	stdoutBuffer := stream.NewBuffer(StreamBufferSize)
	stderrBuffer := stream.NewBuffer(StreamBufferSize)

	timeout := runner.timeout()
	meter := time.After(meterPeriod)
	runTimer := time.After(2 * time.Second)
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
		case <-meter:
			runner.meter()
			meter = time.After(meterPeriod)
		case <-runTimer:
			//if process is still running after the timer has passed, we consider it running
			if !runner.hooksOnExit {
				runner.callHooks(true)
			}
		case message := <-channel:
			if utils.In(stream.ResultMessageLevels, message.Level) {
				result = message
			} else if message.Level == stream.LevelExitState {
				jobresult.State = message.Message
				break loop
			} else if message.Level == stream.LevelStdout {
				stdoutBuffer.Append(message.Message)
			} else if message.Level == stream.LevelStderr {
				stderrBuffer.Append(message.Message)
			} else if message.Level == stream.LevelStatsd {
				runner.statsd.Feed(strings.Trim(message.Message, " "))
			} else if message.Level == stream.LevelCritical {
				critical = message.Message
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

	jobresult.Critical = critical

	return jobresult
}

func (runner *runnerImpl) Run() {
	runs := 0
	var result *core.JobResult
	defer func() {
		runner.statsd.Stop()
		if result != nil {
			runner.result = result
			runner.manager.resultCallback(runner.command, result)

			runner.waitOnce.Do(func() {
				runner.wg.Done()
			})
		}

		runner.manager.cleanUp(runner)
	}()

	//start statsd
	runner.statsd.Run()
loop:
	for {
		result = runner.run()
		if result.State == core.StateSuccess {
			runner.callHooks(true)
		}

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
				log.Infof("Restarting '%s' due to upnormal exit status, trials: %d/%d", runner.command, runs+1, maxRestart)
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
			log.Infof("Recurring '%s' in %d", runner.command, restartIn)
			select {
			case <-time.After(restartIn):
			case <-runner.kill:
				log.Infof("Command %s Killed during scheduler sleep", runner.command)
				result.State = core.StateKilled
				break loop
			}
		} else {
			break
		}
	}

	runner.callHooks(false)
}

func (runner *runnerImpl) callHooks(s bool) {
	runner.hookOnce.Do(func() {
		for _, hook := range runner.hooks {
			hook(s)
		}
	})
}

func (runner *runnerImpl) Kill() {
	runner.kill <- 1
}

func (runner *runnerImpl) Process() process.Process {
	return runner.process
}

func (runner *runnerImpl) Wait() *core.JobResult {
	runner.wg.Wait()
	return runner.result
}
