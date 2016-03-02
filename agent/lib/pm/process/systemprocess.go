package process

import (
	"fmt"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/pm/stream"
	psutils "github.com/shirou/gopsutil/process"
	"log"
	"os"
	"os/exec"
	"path"
)

type systemProcessImpl struct {
	cmd      *core.Cmd
	pid      int
	process  *psutils.Process
	children []*psutils.Process

	table PIDTable
}

func NewSystemProcess(table PIDTable, cmd *core.Cmd) Process {
	return &systemProcessImpl{
		cmd:      cmd,
		children: make([]*psutils.Process, 0),
		table:    table,
	}
}

func (process *systemProcessImpl) Cmd() *core.Cmd {
	return process.cmd
}

func (process *systemProcessImpl) Kill() {
	//should force system process to exit.
	if process.process != nil {
		process.process.Kill()
	}

	process.killChildren()
}

//GetStats gets stats of an external process
func (process *systemProcessImpl) GetStats() *ProcessStats {
	stats := ProcessStats{}
	stats.Cmd = process.cmd

	defer func() {
		if r := recover(); r != nil {
			log.Println("processUtils panic", r)
		}
	}()

	ps := process.process
	if ps == nil {
		return &stats
	}

	cpu, err := ps.CPUPercent(0)
	if err == nil {
		stats.CPU = cpu
	}

	mem, err := ps.MemoryInfo()
	if err == nil {
		stats.RSS = mem.RSS
		stats.VMS = mem.VMS
		stats.Swap = mem.Swap
	}

	stats.Debug = fmt.Sprintf("%d", process.process.Pid)

	for i := 0; i < len(process.children); i++ {
		child := process.children[i]

		childCPU, err := child.CPUPercent(0)
		if err != nil {
			log.Println(err)
			//remove the dead process.
			process.children = append(process.children[:i], process.children[i+1:]...)
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

	return &stats
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

func (process *systemProcessImpl) getExtraEnv() []string {
	env := make([]string, 0, 10)
	agentHome, _ := os.Getwd()
	env = append(env,
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
		fmt.Sprintf("AGENT_HOME=%s", agentHome),
		fmt.Sprintf("AGENT_GID=%d", process.cmd.Gid),
		fmt.Sprintf("AGENT_NID=%d", process.cmd.Nid))

	ctrl := process.cmd.Args.GetController()
	if ctrl == nil {
		return env
	}

	env = append(env,
		fmt.Sprintf("AGENT_CONTROLLER_URL=%s", ctrl.URL),
		fmt.Sprintf("AGENT_CONTROLLER_NAME=%s", process.cmd.Args.GetTag()),
		fmt.Sprintf("AGENT_CONTROLLER_CA=%s", joinCertPath(agentHome, ctrl.Security.CertificateAuthority)),
		fmt.Sprintf("AGENT_CONTROLLER_CLIENT_CERT=%s", joinCertPath(agentHome, ctrl.Security.ClientCertificate)),
		fmt.Sprintf("AGENT_CONTROLLER_CLIENT_CERT_KEY=%s", joinCertPath(agentHome, ctrl.Security.ClientCertificateKey)))

	return env
}

func (process *systemProcessImpl) processInternalMessage(msg *stream.Message) {
	if msg.Level == stream.LevelInternalMonitorPid {
		childPid := 0
		_, err := fmt.Sscanf(msg.Message, "%d", &childPid)
		if err != nil {
			// wrong message format, just ignore.
			return
		}
		log.Println("Tracking external process:", childPid)
		child, err := psutils.NewProcess(int32(childPid))
		if err != nil {
			log.Println(err)
		}
		process.children = append(process.children, child)
	}
}

func (process *systemProcessImpl) killChildren() {
	for _, child := range process.children {
		//kill grand-child process.
		log.Println("Killing grandchild process", child.Pid)

		err := child.Kill()
		if err != nil {
			log.Println("Failed to kill child process", err)
		}
	}
}

func (process *systemProcessImpl) Run() (<-chan *stream.Message, error) {

	args := process.cmd.Args
	cmd := exec.Command(args.GetString("name"),
		args.GetStringArray("args")...)
	cmd.Dir = args.GetString("working_dir")

	extraEnv := process.getExtraEnv()

	env := append(args.GetStringArray("env"),
		extraEnv...)

	if len(env) > 0 {
		cmd.Env = env
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	//starttime := time.Duration(time.Now().UnixNano()) / time.Millisecond // start time in msec
	err = process.table.Register(func() (int, error) {
		err := cmd.Start()
		if err != nil {
			return 0, err
		}

		return cmd.Process.Pid, nil
	})

	if err != nil {
		return nil, err
	}

	channel := make(chan *stream.Message)

	process.pid = cmd.Process.Pid
	psProcess, _ := psutils.NewProcess(int32(process.pid))
	process.process = psProcess

	msgInterceptor := func(msg *stream.Message) {
		if msg.Level == stream.LevelExitState {
			//the level exit state is for internal use only, shouldn't
			//be sent by the app itself, if found, we change the level to err.
			msg.Level = stream.LevelStderr
		}

		if msg.Level > stream.LevelInternal {
			process.processInternalMessage(msg)
			return
		}

		channel <- msg
	}

	// start consuming outputs.
	outConsumer := stream.NewConsumer(stdout, 1)
	outConsumer.Consume(msgInterceptor)

	errConsumer := stream.NewConsumer(stderr, 2)
	errConsumer.Consume(msgInterceptor)

	if process.cmd.Data != "" {
		//write data to command stdin.
		_, err = stdin.Write([]byte(process.cmd.Data))
		if err != nil {
			log.Println("Failed to write to process stdin", err)
		}
	}

	stdin.Close()

	go func(channel chan *stream.Message) {
		//make sure all outputs are closed before waiting for the process
		//to exit.
		defer close(channel)

		<-outConsumer.Signal()
		<-errConsumer.Signal()
		state := process.table.Wait(process.pid)

		log.Println("Process ", process.cmd, " exited with state", state.ExitStatus())

		if state.ExitStatus() == 0 {
			channel <- stream.MessageExitSuccess
		} else {
			channel <- stream.MessageExitError
		}
	}(channel)

	return channel, nil
}
