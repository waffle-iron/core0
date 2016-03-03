package pm

import (
	"fmt"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/pm/process"
	"github.com/g8os/core/agent/lib/pm/stream"
	"github.com/g8os/core/agent/lib/settings"
	"github.com/g8os/core/agent/lib/stats"
	"github.com/g8os/core/agent/lib/utils"
	psutil "github.com/shirou/gopsutil/process"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

//MeterHandler represents a callback type
type MeterHandler func(cmd *core.Cmd, p *psutil.Process)

type MessageHandler func(*core.Cmd, *stream.Message)

//StatsdMeterHandler represents a callback type
type StatsdMeterHandler func(statsd *stats.Statsd, cmd *core.Cmd, p *psutil.Process)

//ResultHandler represents a callback type
type ResultHandler func(cmd *core.Cmd, result *core.JobResult)

//StatsFlushHandler represents a callback type
type StatsFlushHandler func(stats *stats.Stats)

//PM is the main process manager.
type PM struct {
	mid      uint32
	midfile  string
	midMux   sync.Mutex
	cmds     chan *core.Cmd
	runners  map[string]Runner
	statsdes map[string]*stats.Statsd
	maxJobs  int
	jobsCond *sync.Cond

	msgHandlers        []MessageHandler
	resultHandlers     []ResultHandler
	statsFlushHandlers []StatsFlushHandler
	queueMgr           *cmdQueueManager

	pids    map[int]chan *syscall.WaitStatus
	pidsMux sync.Mutex
}

var pm *PM

//NewPM creates a new PM
func NewPM(midfile string, maxJobs int) *PM {
	pm = &PM{
		cmds:     make(chan *core.Cmd),
		midfile:  midfile,
		mid:      loadMid(midfile),
		runners:  make(map[string]Runner),
		maxJobs:  maxJobs,
		jobsCond: sync.NewCond(&sync.Mutex{}),

		msgHandlers:        make([]MessageHandler, 0, 3),
		resultHandlers:     make([]ResultHandler, 0, 3),
		statsFlushHandlers: make([]StatsFlushHandler, 0, 3),
		queueMgr:           newCmdQueueManager(),

		pids: make(map[int]chan *syscall.WaitStatus),
	}

	return pm
}

//TODO: That's not clean, find another way to make this available for other
//code
func GetManager() *PM {
	return pm
}

func loadMid(midfile string) uint32 {
	content, err := ioutil.ReadFile(midfile)
	if err != nil {
		log.Println(err)
		return 0
	}
	v, err := strconv.ParseUint(string(content), 10, 32)
	if err != nil {
		log.Println(err)
		return 0
	}
	return uint32(v)
}

func saveMid(midfile string, mid uint32) {
	ioutil.WriteFile(midfile, []byte(fmt.Sprintf("%d", mid)), 0644)
}

//RunCmd runs and manage command
func (pm *PM) RunCmd(cmd *core.Cmd) {
	pm.cmds <- cmd
}

/*
RunCmdQueued Same as RunCmd put will queue the command for later execution when there are no
other commands runs on the same queue.

The queue name is retrieved from cmd.Args[queue]
*/
func (pm *PM) RunCmdQueued(cmd *core.Cmd) {
	pm.queueMgr.Push(cmd)
}

func (pm *PM) getNextMsgID() uint32 {
	pm.midMux.Lock()
	defer pm.midMux.Unlock()
	pm.mid++
	saveMid(pm.midfile, pm.mid)
	return pm.mid
}

//AddMessageHandler adds handlers for messages that are captured from sub processes. Logger can use this to
//process messages
func (pm *PM) AddMessageHandler(handler MessageHandler) {
	pm.msgHandlers = append(pm.msgHandlers, handler)
}

//AddResultHandler adds a handler that receives job results.
func (pm *PM) AddResultHandler(handler ResultHandler) {
	pm.resultHandlers = append(pm.resultHandlers, handler)
}

//AddStatsFlushHandler adds handler to stats flush.
func (pm *PM) AddStatsFlushHandler(handler StatsFlushHandler) {
	pm.statsFlushHandlers = append(pm.statsFlushHandlers, handler)
}

func (pm *PM) runCmd(cmd *core.Cmd, hooksOnExit bool, hooks ...RunnerHook) Runner {
	factory := GetProcessFactory(cmd)
	//process := NewProcess(cmd)

	if factory == nil {
		log.Println("Unknow command", cmd.Name)
		errResult := core.NewBasicJobResult(cmd)
		errResult.State = core.StateUnknownCmd
		pm.resultCallback(cmd, errResult)
		return nil
	}

	_, exists := pm.runners[cmd.ID]
	if exists {
		errResult := core.NewBasicJobResult(cmd)
		errResult.State = core.StateDuplicateID
		errResult.Data = "A job exists with the same ID"
		pm.resultCallback(cmd, errResult)
		return nil
	}

	runner := NewRunner(pm, cmd, factory, hooksOnExit, hooks...)
	pm.runners[cmd.ID] = runner

	go runner.Run()

	return runner
}

func (pm *PM) processCmds() {
	for {
		pm.jobsCond.L.Lock()

		for len(pm.runners) >= pm.maxJobs {
			pm.jobsCond.Wait()
		}
		pm.jobsCond.L.Unlock()

		var cmd *core.Cmd

		//we have 2 possible sources of cmds.
		//1- cmds that doesn't require waiting on a queue, those can run immediately
		//2- cmds that were waiting on a queue (so they must execute serially)
		select {
		case cmd = <-pm.cmds:
		case cmd = <-pm.queueMgr.Producer():
		}

		pm.runCmd(cmd, false)
	}
}

func (pm *PM) processWait() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGCHLD)
	for _ = range ch {
		var status syscall.WaitStatus
		var rusage syscall.Rusage

		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, &rusage)
		if err != nil {
			log.Printf("wait error %s\n", err)
			continue
		}

		//Avoid reading the process state before the Register call is complete.
		pm.pidsMux.Lock()
		ch, ok := pm.pids[pid]
		pm.pidsMux.Unlock()

		if ok {
			go func() {
				ch <- &status
				close(ch)
				delete(pm.pids, pid)
			}()
		}
	}
}

func (pm *PM) Register(g process.GetPID) error {
	pm.pidsMux.Lock()
	defer pm.pidsMux.Unlock()
	pid, err := g()
	if err != nil {
		return err
	}

	ch := make(chan *syscall.WaitStatus)
	pm.pids[pid] = ch

	return nil
}

func (pm *PM) Wait(pid int) *syscall.WaitStatus {
	return <-pm.pids[pid]
}

//Run starts the process manager.
func (pm *PM) Run() {
	//process and start all commands according to args.
	go pm.processWait()
	go pm.processCmds()
}

/*
RunSlice runs a slice of processes honoring dependencies. It won't just
start in order, but will also make sure a service won't start until it's dependencies are
running.
*/
func (pm *PM) RunSlice(slice settings.StartupSlice) {
	state := NewStateMachine()
	var wg sync.WaitGroup

	provided := make(map[string]int)
	needed := make(map[string]int)

	for _, startup := range slice {
		log.Println("Startup command ", startup)
		if startup.Args == nil {
			startup.Args = make(map[string]interface{})
		}

		cmd := &core.Cmd{
			Gid:  settings.Options.Gid(),
			Nid:  settings.Options.Nid(),
			ID:   startup.Key(),
			Name: startup.Name,
			Data: startup.Data,
			Args: core.NewMapArgs(startup.Args),
		}

		provided[cmd.ID] = 1
		for _, k := range startup.After {
			needed[k] = 1
		}

		meterInt := cmd.Args.GetInt("stats_interval")
		if meterInt == 0 {
			cmd.Args.Set("stats_interval", settings.Settings.Stats.Interval)
		}

		wg.Add(1)
		go func(up settings.Startup, c *core.Cmd) {
			log.Printf("Waiting for %s to run %s\n", up.After, cmd)
			canRun := state.Wait(up.After...)

			if canRun {
				log.Printf("Starting %s\n", c)
				pm.runCmd(c, up.MustExit, func(s bool) {
					state.Release(c.ID, s)
				})
			} else {
				log.Printf("ERROR: Can't start %s because one of the dependencies failed\n", c)
			}
			wg.Done()
		}(startup, cmd)
	}
	//release all dependencies that are not provided by this slice.
	for k := range needed {
		if _, ok := provided[k]; !ok {
			log.Println("Auto releasing of", k)
			state.Release(k, true)
		}
	}

	wg.Wait()
}

func (pm *PM) cleanUp(runner Runner) {
	delete(pm.runners, runner.Command().ID)

	pm.queueMgr.Notify(runner.Command())
	pm.jobsCond.Broadcast()
}

//Processes returs a list of running processes
func (pm *PM) Runners() map[string]Runner {
	return pm.runners
}

//Killall kills all running processes.
func (pm *PM) Killall() {
	for _, v := range pm.runners {
		go v.Kill()
	}
}

//Kill kills a process by the cmd ID
func (pm *PM) Kill(cmdID string) {
	v, o := pm.runners[cmdID]
	if o {
		v.Kill()
	}
}

func (pm *PM) msgCallback(cmd *core.Cmd, msg *stream.Message) {
	levels := cmd.Args.GetIntArray("loglevels")
	if len(levels) > 0 && !utils.In(levels, msg.Level) {
		return
	}

	//stamp msg.
	msg.Epoch = time.Now().UnixNano()
	//add ID
	msg.ID = pm.getNextMsgID()
	for _, handler := range pm.msgHandlers {
		handler(cmd, msg)
	}
}

func (pm *PM) resultCallback(cmd *core.Cmd, result *core.JobResult) {
	result.Tags = cmd.Tags
	result.Args = cmd.Args

	for _, handler := range pm.resultHandlers {
		handler(cmd, result)
	}
}

func (pm *PM) statsFlushCallback(stats *stats.Stats) {
	for _, handler := range pm.statsFlushHandlers {
		handler(stats)
	}
}
