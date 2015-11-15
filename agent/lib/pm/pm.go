package pm

import (
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	//"github.com/Jumpscale/agent2/agent/lib/pm/process"
	"github.com/Jumpscale/agent2/agent/lib/pm/stream"
	"github.com/Jumpscale/agent2/agent/lib/stats"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	psutil "github.com/shirou/gopsutil/process"
	"io/ioutil"
	"log"
	"strconv"
	"sync"
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
	midMux   *sync.Mutex
	cmds     chan *core.Cmd
	runners  map[string]Runner
	statsdes map[string]*stats.Statsd
	maxJobs  int
	jobsCond *sync.Cond

	msgHandlers        []MessageHandler
	resultHandlers     []ResultHandler
	statsFlushHandlers []StatsFlushHandler
	queueMgr           *cmdQueueManager
}

var pm *PM

//NewPM creates a new PM
func NewPM(midfile string, maxJobs int) *PM {
	pm = &PM{
		cmds:     make(chan *core.Cmd),
		midfile:  midfile,
		mid:      loadMid(midfile),
		midMux:   &sync.Mutex{},
		runners:  make(map[string]Runner),
		maxJobs:  maxJobs,
		jobsCond: sync.NewCond(&sync.Mutex{}),

		msgHandlers:        make([]MessageHandler, 0, 3),
		resultHandlers:     make([]ResultHandler, 0, 3),
		statsFlushHandlers: make([]StatsFlushHandler, 0, 3),
		queueMgr:           newCmdQueueManager(),
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

//Run starts the process manager.
func (pm *PM) Run() {
	//process and start all commands according to args.
	go func() {
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

			factory := GetProcessFactory(cmd)
			//process := NewProcess(cmd)

			if factory == nil {
				log.Println("Unknow command", cmd.Name)
				errResult := core.NewBasicJobResult(cmd)
				errResult.State = core.StateUnknownCmd
				pm.resultCallback(cmd, errResult)
				continue
			}

			_, exists := pm.runners[cmd.ID]
			if exists {
				errResult := core.NewBasicJobResult(cmd)
				errResult.State = core.StateDuplicateID
				errResult.Data = "A job exists with the same ID"
				pm.resultCallback(cmd, errResult)
				continue
			}

			runner := NewRunner(pm, cmd, factory)
			pm.runners[cmd.ID] = runner

			// A process must signal it's termination (that it's not going
			// to restart) for the process manager to clean up it's reference
			signal := make(chan int)
			go func() {
				<-signal
				close(signal)
				delete(pm.runners, cmd.ID)
				delete(pm.statsdes, cmd.ID)

				//tell the queue that this command has finished so it prepares a
				//new command to execute
				pm.queueMgr.Notify(cmd)

				//tell manager that there is a process slot ready.
				pm.jobsCond.Broadcast()
			}()

			go runner.Run()
		}
	}()
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
