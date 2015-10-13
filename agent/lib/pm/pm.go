package pm

import (
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/stats"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"github.com/shirou/gopsutil/process"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

//Cmd is an executable command
type Cmd struct {
	ID    string   `json:"id"`
	Gid   int      `json:"gid"`
	Nid   int      `json:"nid"`
	Roles []string `json:"roles"`
	Name  string   `json:"cmd"`
	Args  *MapArgs `json:"args"`
	Data  string   `json:"data"`
}

//NewMapCmd builds a cmd from a map.
func NewMapCmd(data map[string]interface{}) *Cmd {
	stdin, ok := data["data"]
	if !ok {
		stdin = ""
	}
	cmd := &Cmd{
		Gid:  data["gid"].(int),
		Nid:  data["nid"].(int),
		ID:   data["id"].(string),
		Name: data["name"].(string),
		Data: stdin.(string),
		Args: NewMapArgs(data["args"].(map[string]interface{})),
	}

	return cmd
}

//LoadCmd loads cmd from json string.
func LoadCmd(str []byte) (*Cmd, error) {
	cmd := new(Cmd)
	err := json.Unmarshal(str, cmd)
	if err != nil {
		return nil, err
	}
	if cmd.Args == nil || cmd.Args.data == nil {
		cmd.Args = NewMapArgs(map[string]interface{}{})
	}

	return cmd, err
}

//String represents cmd as a string
func (cmd *Cmd) String() string {
	return fmt.Sprintf("(%s# %s %s)", cmd.ID, cmd.Name, cmd.Args.GetString("name"))
}

//MeterHandler represents a callback type
type MeterHandler func(cmd *Cmd, p *process.Process)

//StatsdMeterHandler represents a callback type
type StatsdMeterHandler func(statsd *stats.Statsd, cmd *Cmd, p *process.Process)

//MessageHandler represents a callback type
type MessageHandler func(msg *Message)

//ResultHandler represents a callback type
type ResultHandler func(result *JobResult)

//StatsFlushHandler represents a callback type
type StatsFlushHandler func(stats *stats.Stats)

//PM is the main process manager.
type PM struct {
	mid       uint32
	midfile   string
	midMux    *sync.Mutex
	cmds      chan *Cmd
	processes map[string]Process
	statsdes  map[string]*stats.Statsd
	maxJobs   int
	jobsCond  *sync.Cond

	statsdMeterHandlers []StatsdMeterHandler
	msgHandlers         []MessageHandler
	resultHandlers      []ResultHandler
	statsFlushHandlers  []StatsFlushHandler
	queueMgr            *CmdQueueManager
}

//NewPM creates a new PM
func NewPM(midfile string, maxJobs int) *PM {
	pm := &PM{
		cmds:      make(chan *Cmd),
		midfile:   midfile,
		mid:       loadMid(midfile),
		midMux:    &sync.Mutex{},
		processes: make(map[string]Process),
		statsdes:  make(map[string]*stats.Statsd),
		maxJobs:   maxJobs,
		jobsCond:  sync.NewCond(&sync.Mutex{}),

		statsdMeterHandlers: make([]StatsdMeterHandler, 0, 3),
		msgHandlers:         make([]MessageHandler, 0, 3),
		resultHandlers:      make([]ResultHandler, 0, 3),
		statsFlushHandlers:  make([]StatsFlushHandler, 0, 3),
		queueMgr:            NewCmdQueueManager(),
	}
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
func (pm *PM) RunCmd(cmd *Cmd) {
	pm.cmds <- cmd
}

/*
RunCmdQueued Same as RunCmd put will queue the command for later execution when there are no
other commands runs on the same queue.

The queue name is retrieved from cmd.Args[queue]
*/
func (pm *PM) RunCmdQueued(cmd *Cmd) {
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

			for len(pm.processes) >= pm.maxJobs {
				pm.jobsCond.Wait()
			}
			pm.jobsCond.L.Unlock()

			var cmd *Cmd

			//we have 2 possible sources of cmds.
			//1- cmds that doesn't require waiting on a queue, those can run immediately
			//2- cmds that were waiting on a queue (so they must execute serially)
			select {
			case cmd = <-pm.cmds:
			case cmd = <-pm.queueMgr.Producer():
			}

			process := NewProcess(cmd)

			if process == nil {
				log.Println("Unknow command", cmd.Name)
				errResult := NewBasicJobResult(cmd)
				errResult.State = StateUnknownCmd
				pm.resultCallback(errResult)
				continue
			}

			_, exists := pm.processes[cmd.ID]
			if exists {
				errResult := NewBasicJobResult(cmd)
				errResult.State = StateDuplicateId
				errResult.Data = "A job exists with the same ID"
				pm.resultCallback(errResult)
				continue
			}

			pm.processes[cmd.ID] = process

			statsInterval := cmd.Args.GetInt("stats_interval")

			prefix := fmt.Sprintf("%d.%d.%s.%s.%s", cmd.Gid, cmd.Nid, cmd.Name,
				cmd.Args.GetString("domain"), cmd.Args.GetString("name"))

			statsd := stats.NewStatsd(
				prefix,
				time.Duration(statsInterval)*time.Second,
				pm.statsFlushCallback)

			statsd.Run()
			pm.statsdes[cmd.ID] = statsd

			// A process must signal it's termination (that it's not going
			// to restart) for the process manager to clean up it's reference
			signal := make(chan int)
			go func() {
				<-signal
				close(signal)
				statsd.Stop()
				delete(pm.processes, cmd.ID)
				delete(pm.statsdes, cmd.ID)

				//tell the queue that this command has finished so it prepares a
				//new command to execute
				pm.queueMgr.Notify(cmd)

				//tell manager that there is a process slot ready.
				pm.jobsCond.Broadcast()
			}()

			go process.Run(RunCfg{
				ProcessManager: pm,
				MeterHandler:   pm.meterCallback,
				MessageHandler: pm.msgCallback,
				ResultHandler:  pm.resultCallback,
				Signal:         signal,
			})
		}
	}()
}

//Processes returs a list of running processes
func (pm *PM) Processes() map[string]Process {
	return pm.processes
}

//Killall kills all running processes.
func (pm *PM) Killall() {
	for _, v := range pm.processes {
		go v.Kill()
	}
}

//Kill kills a process by the cmd ID
func (pm *PM) Kill(cmdID string) {
	v, o := pm.processes[cmdID]
	if o {
		v.Kill()
	}
}

func (pm *PM) meterCallback(cmd *Cmd, ps *process.Process) {
	statsd, ok := pm.statsdes[cmd.ID]
	if !ok {
		return
	}

	cpu, err := ps.CPUPercent(0)
	if err == nil {
		statsd.Gauage("_cpu_", fmt.Sprintf("%f", cpu))
	}

	mem, err := ps.MemoryInfo()
	if err == nil {
		statsd.Gauage("_rss_", fmt.Sprintf("%d", mem.RSS))
		statsd.Gauage("_vms_", fmt.Sprintf("%d", mem.VMS))
		statsd.Gauage("_swap_", fmt.Sprintf("%d", mem.Swap))
	}
}

func (pm *PM) handlStatsdMsgs(msg *Message) {
	statsd, ok := pm.statsdes[msg.Cmd.ID]
	if !ok {
		// there is no statsd configured for this process!! we shouldn't
		// be here but just in case
		return
	}

	statsd.Feed(strings.Trim(msg.Message, " "))
}
func (pm *PM) msgCallback(msg *Message) {
	if msg.Level == LevelStatsd {
		pm.handlStatsdMsgs(msg)
	}

	levels := msg.Cmd.Args.GetIntArray("loglevels")
	if len(levels) > 0 && !utils.In(levels, msg.Level) {
		return
	}

	//stamp msg.
	msg.Epoch = time.Now().UnixNano()
	//add ID
	msg.ID = pm.getNextMsgID()
	for _, handler := range pm.msgHandlers {
		handler(msg)
	}
}

func (pm *PM) resultCallback(result *JobResult) {
	for _, handler := range pm.resultHandlers {
		handler(result)
	}
}

func (pm *PM) statsFlushCallback(stats *stats.Stats) {
	for _, handler := range pm.statsFlushHandlers {
		handler(stats)
	}
}
