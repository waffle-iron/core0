package pm

import (
	"bytes"
	"container/list"
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"github.com/shirou/gopsutil/process"
	"log"
	"os"
	"os/exec"
	"path"
	"time"
)

const (
	//LevelStdout stdout message
	LevelStdout = 1 // stdout
	//LevelStderr stderr message
	LevelStderr = 2 // stderr
	//LevelPublic public message
	LevelPublic = 3 // message for endusers / public message
	//LevelOperator operator message
	LevelOperator = 4 // message for operator / internal message
	//LevelUnknown unknown message
	LevelUnknown = 5 // log msg (unstructured = level5, cat=unknown)
	//LevelStructured structured message
	LevelStructured = 6 // log msg structured
	//LevelWarning warning message
	LevelWarning = 7 // warning message
	//LevelOpsError ops error message
	LevelOpsError = 8 // ops error
	//LevelCritical critical message
	LevelCritical = 9 // critical error
	//LevelStatsd statsd message
	LevelStatsd = 10 // statsd message(s) AVG
	//LevelDebug debug message
	LevelDebug = 11 // debug message
	//LevelResultJSON json result message
	LevelResultJSON = 20 // result message, json
	//LevelResultYAML yaml result message
	LevelResultYAML = 21 // result message, yaml
	//LevelResultTOML toml result message
	LevelResultTOML = 22 // result message, toml
	//LevelResultHRD hrd result message
	LevelResultHRD = 23 // result message, hrd
	//LevelResultJob job result message
	LevelResultJob = 30 // job, json (full result of a job)

	//LevelInternal specify the start of the internal log levels
	LevelInternal = 100

	//LevelInternalMonitorPid instruct the agent to consider the cpu and mem consumption
	//of that PID (in the message body)
	LevelInternalMonitorPid = 101

	//StateSuccess successs exit status
	StateSuccess = "SUCCESS"
	//StateError error exist status
	StateError = "ERROR"
	//StateTimeout timeout exit status
	StateTimeout = "TIMEOUT"
	//StateKilled killed exit status
	StateKilled = "KILLED"
	//StateUnknownCmd unknown cmd exit status
	StateUnknownCmd = "UNKNOWN_CMD"
	//StateDuplicateID dublicate id exit status
	StateDuplicateID = "DUPILICATE_ID"

	//StreamBufferSize max number of lines to capture from a stream
	StreamBufferSize = 1000 // keeps only last 1000 line of stream
)

var resultMessageLevels = []int{LevelResultJSON,
	LevelResultYAML, LevelResultTOML, LevelResultHRD, LevelResultJob}

//ProcessStats holds process cpu and memory usage
type ProcessStats struct {
	Cmd   *Cmd    `json:"cmd"`
	CPU   float64 `json:"cpu"`
	RSS   uint64  `json:"rss"`
	VMS   uint64  `json:"vms"`
	Swap  uint64  `json:"swap"`
	Debug string  `json:"debug,ommitempty"`
}

//Process interface
type Process interface {
	Cmd() *Cmd
	Run(RunCfg)
	Kill()
	GetStats() *ProcessStats
}

//RunCfg holds configuration and callbacks to be passed to a running process so the process can feed the process manager with messages.
//and results
type RunCfg struct {
	ProcessManager *PM
	MeterHandler   MeterHandler
	MessageHandler MessageHandler
	ResultHandler  ResultHandler
	Signal         chan int
}

//JobResult represents a result of a job
type JobResult struct {
	ID        string   `json:"id"`
	Gid       int      `json:"gid"`
	Nid       int      `json:"nid"`
	Cmd       string   `json:"cmd"`
	Args      *MapArgs `json:"args"`
	Data      string   `json:"data"`
	Streams   []string `json:"streams,omitempty"`
	Critical  string   `json:"critical,omitempty"`
	Level     int      `json:"level"`
	State     string   `json:"state"`
	StartTime int64    `json:"starttime"`
	Time      int64    `json:"time"`
	Tags      string   `json:"tags"`
}

//NewBasicJobResult creates a new job result from command
func NewBasicJobResult(cmd *Cmd) *JobResult {
	return &JobResult{
		ID:   cmd.ID,
		Gid:  cmd.Gid,
		Nid:  cmd.Nid,
		Cmd:  cmd.Name,
		Args: cmd.Args,
	}
}

//Message is a message from running process
type Message struct {
	ID      uint32
	Cmd     *Cmd
	Level   int
	Message string
	Epoch   int64
}

//MarshalJSON serializes message to json
func (msg *Message) MarshalJSON() ([]byte, error) {
	data := make(map[string]interface{})
	args := msg.Cmd.Args
	data["jobid"] = msg.Cmd.ID
	data["domain"] = args.GetString("domain")
	data["name"] = args.GetString("name")
	data["epoch"] = msg.Epoch / int64(time.Millisecond)
	data["level"] = msg.Level
	data["id"] = msg.ID
	data["data"] = msg.Message

	return json.Marshal(data)
}

//String represents a message as a string
func (msg *Message) String() string {
	return fmt.Sprintf("%s|%d:%s", msg.Cmd, msg.Level, msg.Message)
}

//ExtProcess represents an external process
type ExtProcess struct {
	cmd      *Cmd
	ctrl     chan int
	pid      int
	runs     int
	process  *process.Process
	children []*process.Process
}

//NewExtProcess creates a new external process from a command
func NewExtProcess(cmd *Cmd) Process {
	return &ExtProcess{
		cmd:      cmd,
		ctrl:     make(chan int),
		children: make([]*process.Process, 0),
	}
}

//Cmd returns the command
func (ps *ExtProcess) Cmd() *Cmd {
	return ps.cmd
}

func concatBuffer(buffer *list.List) string {
	var strbuf bytes.Buffer
	for l := buffer.Front(); l != nil; l = l.Next() {
		strbuf.WriteString(l.Value.(string))
		strbuf.WriteString("\n")
	}

	return strbuf.String()
}

func joinCertPath(base string, relative string) string {
	if relative == "" {
		return relative
	}

	if path.IsAbs(relative) {
		return relative
	}

	return path.Join(base, relative)
}

func (ps *ExtProcess) getExtraEnv() []string {
	env := make([]string, 0, 10)
	agentHome, _ := os.Getwd()
	env = append(env,
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
		fmt.Sprintf("AGENT_HOME=%s", agentHome),
		fmt.Sprintf("AGENT_GID=%d", ps.cmd.Gid),
		fmt.Sprintf("AGENT_NID=%d", ps.cmd.Nid))

	ctrl := ps.cmd.Args.GetController()
	if ctrl == nil {
		return env
	}

	env = append(env,
		fmt.Sprintf("AGENT_CONTROLLER_URL=%s", ctrl.URL),
		fmt.Sprintf("AGENT_CONTROLLER_NAME=%s", ps.cmd.Args.GetTag()),
		fmt.Sprintf("AGENT_CONTROLLER_CA=%s", joinCertPath(agentHome, ctrl.Security.CertificateAuthority)),
		fmt.Sprintf("AGENT_CONTROLLER_CLIENT_CERT=%s", joinCertPath(agentHome, ctrl.Security.ClientCertificate)),
		fmt.Sprintf("AGENT_CONTROLLER_CLIENT_CERT_KEY=%s", joinCertPath(agentHome, ctrl.Security.ClientCertificateKey)))

	return env
}

//Run starts process, feed data over the process stdin, and start
//consuming both stdout, and stderr.
//All messages from the subprocesses are
func (ps *ExtProcess) Run(cfg RunCfg) {
	args := ps.cmd.Args
	cmd := exec.Command(args.GetString("name"),
		args.GetStringArray("args")...)
	cmd.Dir = args.GetString("working_dir")

	extraEnv := ps.getExtraEnv()

	env := append(args.GetStringArray("env"),
		extraEnv...)

	if len(env) > 0 {
		cmd.Env = env
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println("Failed to open process stdout", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Println("Failed to open process stderr", err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Println("Failed to open process stdin", err)
	}

	starttime := time.Duration(time.Now().UnixNano()) / time.Millisecond // start time in msec

	restarting := false
	defer func() {
		if !restarting {
			close(ps.ctrl)
			cfg.Signal <- 1 // forces the PM to clean up
		}
	}()

	err = cmd.Start()
	if err != nil {
		log.Println("Failed to start process", err)
		jobresult := NewBasicJobResult(ps.cmd)
		jobresult.State = StateError
		jobresult.Data = fmt.Sprintf("%v", err)
		cfg.ResultHandler(ps.cmd, jobresult)
		return
	}

	ps.pid = cmd.Process.Pid
	psProcess, _ := process.NewProcess(int32(ps.pid))
	ps.process = psProcess

	var result *Message

	stdoutBuffer := list.New()
	stderrBuffer := list.New()
	var critical string

	msgInterceptor := func(msg *Message) {
		if utils.In(resultMessageLevels, msg.Level) {
			//process result message.
			result = msg
		}

		if msg.Level > LevelInternal {
			ps.processInternalMessage(msg)
		} else if msg.Level == LevelStdout {
			stdoutBuffer.PushBack(msg.Message)
			if stdoutBuffer.Len() > StreamBufferSize {
				stdoutBuffer.Remove(stdoutBuffer.Front())
			}
		} else if msg.Level == LevelStderr {
			stderrBuffer.PushBack(msg.Message)
			if stderrBuffer.Len() > StreamBufferSize {
				stderrBuffer.Remove(stderrBuffer.Front())
			}
		} else if msg.Level == LevelCritical {
			critical = msg.Message
		}

		cfg.MessageHandler(msg)
	}

	// start consuming outputs.
	outConsumer := newStreamConsumer(ps.cmd, stdout, 1)
	outConsumer.Consume(msgInterceptor)

	errConsumer := newStreamConsumer(ps.cmd, stderr, 2)
	errConsumer.Consume(msgInterceptor)

	if ps.cmd.Data != "" {
		//write data to command stdin.
		_, err = stdin.Write([]byte(ps.cmd.Data))
		if err != nil {
			log.Println("Failed to write to process stdin", err)
		}
	}

	stdin.Close()

	psexit := make(chan bool)

	go func() {
		//make sure all outputs are closed before waiting for the process
		//to exit.
		<-outConsumer.Signal
		<-errConsumer.Signal

		err := cmd.Wait()
		if err != nil {
			log.Println(err)
		}
		psexit <- cmd.ProcessState.Success()
	}()

	var timeout <-chan time.Time

	if args.GetInt("max_time") > 0 {
		timeout = time.After(time.Duration(args.GetInt("max_time")) * time.Second)
	}

	var success bool
	var timedout bool
	var killed bool

loop:
	for {
		select {
		case success = <-psexit:
			//handle process exit
			log.Println("process exited normally")
			break loop
		case <-timeout:
			//process timed out.
			log.Println("process timed out")
			ps.killChildren()
			cmd.Process.Kill()
			timedout = true
		case s := <-ps.ctrl:
			if s == 1 {
				//kill signal
				log.Println("killing process", ps.cmd, cmd.Process.Pid)
				cmd.Process.Kill()
				killed = true
				ps.runs = 0
			}
		case <-time.After(30 * time.Second):
			//monitor.
			cfg.MeterHandler(ps.cmd, psProcess)
		}
	}

	endtime := time.Duration(time.Now().UnixNano()) / time.Millisecond

	if endtime-starttime > 300*time.Millisecond {
		//if process lived for more than 5 min before it dies, reset the runs
		//this means that only the max_restart count will be reached if the
		//process kept failing under the 5 min limit.
		ps.runs = 0
	}

	//process exited.
	log.Println(ps.cmd, "exit status: ", success)

	if !killed {
		//only can restart if processes wasn't killed
		var restartIn time.Duration

		//restarting due to failuer with max_restart set
		if !success && args.GetInt("max_restart") > 0 {
			ps.runs++
			if ps.runs < args.GetInt("max_restart") {
				log.Println("Restarting", ps.cmd, "due to upnormal exit status, trials", ps.runs+1, "/", args.GetInt("max_restart"))
				restarting = true
				restartIn = 1 * time.Second
			}
		}

		//recurring task, need to restart anyway
		if args.GetInt("recurring_period") > 0 {
			log.Println("Recurring", ps.cmd, "in", args.GetInt("recurring_period"), "seconds")
			restarting = true
			restartIn = time.Duration(args.GetInt("recurring_period")) * time.Second
		}

		if restarting {
			//we are starting a go routine here. so normal execution of
			//the caller function will be followed.
			go func() {
				select {
				case <-time.After(restartIn):
					if success {
						ps.runs = 0
					} else {
						ps.runs++
					}
					//restarting.
					ps.Run(cfg)
				case s := <-ps.ctrl:
					//process killed while waiting.
					//since the waiting was done inside a go routine. we need to do
					//the normal cleaning, that was skipped because of the restarting flag.
					if s == 1 {
						log.Println(ps.cmd, "killed while waiting for recurring")
						killed = true
						defer func() {
							close(ps.ctrl)
							cfg.Signal <- 1 //forces the PM to clean up
						}()
						//and send the kill result.
						jobresult := NewBasicJobResult(ps.cmd)

						jobresult.State = StateKilled
						jobresult.StartTime = int64(starttime)
						jobresult.Time = int64(endtime - starttime)
						cfg.ResultHandler(ps.cmd, jobresult)
					}
				}
			}()
		}
	}

	var state string
	if success {
		state = StateSuccess
	} else if timedout {
		state = StateTimeout
	} else if killed {
		state = StateKilled
	} else {
		state = StateError
	}

	jobresult := NewBasicJobResult(ps.cmd)

	jobresult.State = state
	jobresult.StartTime = int64(starttime)
	jobresult.Time = int64(endtime - starttime)

	if result != nil {
		jobresult.Data = result.Message
		jobresult.Level = result.Level
	}

	if success && restarting && result == nil {
		//this is a recurring task. No need to flud
		//AC with success status.
		return
	}

	jobresult.Streams = []string{
		concatBuffer(stdoutBuffer),
		concatBuffer(stderrBuffer),
	}

	jobresult.Critical = critical
	//delegating the result.
	cfg.ResultHandler(ps.cmd, jobresult)
}

func (ps *ExtProcess) processInternalMessage(msg *Message) {
	if msg.Level == LevelInternalMonitorPid {
		childPid := 0
		_, err := fmt.Sscanf(msg.Message, "%d", &childPid)
		if err != nil {
			// wrong message format, just ignore.
			return
		}
		log.Println("Tracking external process:", childPid)
		child, err := process.NewProcess(int32(childPid))
		if err != nil {
			log.Println(err)
		}
		ps.children = append(ps.children, child)
	}
}

func (ps *ExtProcess) killChildren() {
	for _, child := range ps.children {
		//kill grand-child process.
		log.Println("Killing grandchild process", child.Pid)

		err := child.Kill()
		if err != nil {
			log.Println("Failed to kill child process", err)
		}
	}
}

//Kill stops an external process
func (ps *ExtProcess) Kill() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Killing job that is already gone", ps.cmd)
		}
	}()

	ps.killChildren()
	//signal child process to terminate
	ps.ctrl <- 1
}

//GetStats gets stats of an external process
func (ps *ExtProcess) GetStats() *ProcessStats {
	stats := new(ProcessStats)
	stats.Cmd = ps.cmd

	cpu, err := ps.process.CPUPercent(0)
	if err == nil {
		stats.CPU = cpu
	}

	mem, err := ps.process.MemoryInfo()
	if err == nil {
		stats.RSS = mem.RSS
		stats.VMS = mem.VMS
		stats.Swap = mem.Swap
	}

	stats.Debug = fmt.Sprintf("%d", ps.process.Pid)

	for i := 0; i < len(ps.children); i++ {
		child := ps.children[i]

		childCPU, err := child.CPUPercent(0)
		if err != nil {
			log.Println(err)
			//remove the dead process.
			ps.children = append(ps.children[:i], ps.children[i+1:]...)
			continue
		}

		stats.CPU += childCPU
		childMem, err := child.MemoryInfo()
		if err == nil {
			stats.Debug = fmt.Sprintf("%s %d", stats.Debug, child.Pid)
			stats.RSS += childMem.RSS
			stats.Swap += childMem.Swap
			stats.VMS += childMem.VMS
		} else {
			log.Println(err)
		}
	}

	return stats
}
