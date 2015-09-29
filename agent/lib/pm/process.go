package pm

import (
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"github.com/shirou/gopsutil/process"
	"log"
	"os"
	"os/exec"
	"time"
)

const (
	L_STDOUT      = 1  // stdout
	L_STDERR      = 2  // stderr
	L_PUBLIC      = 3  // message for endusers / public message
	L_OPERATOR    = 4  // message for operator / internal message
	L_UNKNOWN     = 5  // log msg (unstructured = level5, cat=unknown)
	L_STRUCTURED  = 6  // log msg structured
	L_WARNING     = 7  // warning message
	L_OPS_ERROR   = 8  // ops error
	L_CRITICAL    = 9  // critical error
	L_STATSD      = 10 // statsd message(s) AVG
	L_DEBUG       = 11 // debug message
	L_RESULT_JSON = 20 // result message, json
	L_RESULT_YAML = 21 // result message, yaml
	L_RESULT_TOML = 22 // result message, toml
	L_RESULT_HRD  = 23 // result message, hrd
	L_RESULT_JOB  = 30 // job, json (full result of a job)

	S_SUCCESS      = "SUCCESS"
	S_ERROR        = "ERROR"
	S_TIMEOUT      = "TIMEOUT"
	S_KILLED       = "KILLED"
	S_UNKNOWN_CMD  = "UNKNOWN_CMD"
	S_DUPILCATE_ID = "DUPILICATE_ID"
)

var RESULT_MESSAGE_LEVELS []int = []int{L_RESULT_JSON,
	L_RESULT_YAML, L_RESULT_TOML, L_RESULT_HRD, L_RESULT_JOB}

type ProcessStats struct {
	Cmd  *Cmd    `json:"cmd"`
	CPU  float64 `json:"cpu"`
	RSS  uint64  `json:"rss"`
	VMS  uint64  `json:"vms"`
	Swap uint64  `json:"swap"`
}

type Process interface {
	Cmd() *Cmd
	Run(RunCfg)
	Kill()
	GetStats() *ProcessStats
}

type RunCfg struct {
	ProcessManager *PM
	MeterHandler   MeterHandler
	MessageHandler MessageHandler
	ResultHandler  ResultHandler
	Signal         chan int
}

type JobResult struct {
	Id        string   `json:"id"`
	Gid       int      `json:"gid"`
	Nid       int      `json:"nid"`
	Cmd       string   `json:"cmd"`
	Args      *MapArgs `json:"args"`
	Data      string   `json:"data"`
	Level     int      `json:"level"`
	State     string   `json:"state"`
	StartTime int64    `json:"starttime"`
	Time      int64    `json:"time"`
}

func NewBasicJobResult(cmd *Cmd) *JobResult {
	return &JobResult{
		Id:   cmd.Id,
		Gid:  cmd.Gid,
		Nid:  cmd.Nid,
		Cmd:  cmd.Name,
		Args: cmd.Args,
	}
}

type Message struct {
	Id      uint32
	Cmd     *Cmd
	Level   int
	Message string
	Epoch   int64
}

func (msg *Message) MarshalJSON() ([]byte, error) {
	data := make(map[string]interface{})
	args := msg.Cmd.Args
	data["domain"] = args.GetString("domaing")
	data["name"] = args.GetString("name")
	data["epoch"] = msg.Epoch
	data["level"] = msg.Level
	data["id"] = msg.Id
	data["data"] = msg.Message

	return json.Marshal(data)
}

func (msg *Message) String() string {
	return fmt.Sprintf("%s|%d:%s", msg.Cmd, msg.Level, msg.Message)
}

type ExtProcess struct {
	cmd     *Cmd
	ctrl    chan int
	pid     int
	runs    int
	process *process.Process
}

func NewExtProcess(cmd *Cmd) Process {
	return &ExtProcess{
		cmd:  cmd,
		ctrl: make(chan int),
	}
}

func (ps *ExtProcess) Cmd() *Cmd {
	return ps.cmd
}

//Start process, feed data over the process stdin, and start
//consuming both stdout, and stderr.
//All messages from the subprocesses are
func (ps *ExtProcess) Run(cfg RunCfg) {
	args := ps.cmd.Args
	cmd := exec.Command(args.GetString("name"),
		args.GetStringArray("args")...)
	cmd.Dir = args.GetString("working_dir")

	env := append(args.GetStringArray("env"),
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")))

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
		jobresult.State = S_ERROR
		jobresult.Data = fmt.Sprintf("%v", err)
		cfg.ResultHandler(jobresult)
		return
	}

	ps.pid = cmd.Process.Pid
	psProcess, _ := process.NewProcess(int32(ps.pid))
	ps.process = psProcess

	var result *Message = nil

	msgInterceptor := func(msg *Message) {
		if utils.In(RESULT_MESSAGE_LEVELS, msg.Level) {
			//process result message.
			result = msg
		}

		cfg.MessageHandler(msg)
	}

	// start consuming outputs.
	outConsumer := NewStreamConsumer(ps.cmd, stdout, 1)
	outConsumer.Consume(msgInterceptor)

	errConsumer := NewStreamConsumer(ps.cmd, stderr, 2)
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
			ps.runs += 1
			if ps.runs < args.GetInt("max_restart") {
				log.Println("Restarting", ps.cmd, "due to upnormal exit status, trials", ps.runs, "/", args.GetInt("max_restart"))
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
						ps.runs += 1
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

						jobresult.State = S_KILLED
						jobresult.StartTime = int64(starttime)
						jobresult.Time = int64(endtime - starttime)
						cfg.ResultHandler(jobresult)
					}
				}
			}()
		}
	}

	var state string
	if success {
		state = S_SUCCESS
	} else if timedout {
		state = S_TIMEOUT
	} else if killed {
		state = S_KILLED
	} else {
		state = S_ERROR
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

	//delegating the result.
	cfg.ResultHandler(jobresult)
}

func (ps *ExtProcess) Kill() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Killing job that is already gone", ps)
		}
	}()

	ps.ctrl <- 1
}

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

	return stats
}
