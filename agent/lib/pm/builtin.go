package pm

//implement internal processes

import (
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/Jumpscale/agent2/agent/lib/pm/process"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	//psutil "github.com/shirou/gopsutil/process"
)

const (
	cmdExecute = "execute"
)

/*
ProcessConstructor represnts a function that returns a Process
*/
type ProcessConstructor func(cmd *core.Cmd) process.Process

/*
Global command ProcessConstructor registery
*/
var CmdMap = map[string]ProcessConstructor{
	cmdExecute: NewExtProcess,
}

/*
NewProcess creates a new process from a command
*/
func NewProcess(cmd *core.Cmd) process.Process {
	constructor, ok := CmdMap[cmd.Name]
	if !ok {
		return nil
	}

	return constructor(cmd)
}

type extensionProcess struct {
	extps process.Process
	cmd   *core.Cmd
}

func newExtensionProcess(exe string, workdir string, cmdargs []string, env []string) ProcessConstructor {
	//create a new execute process with python2.7 or lua as executors.
	constructor := func(cmd *core.Cmd) process.Process {
		args := cmd.Args.Clone(false)
		args.Set("name", exe)

		jobCmdArgs := make([]string, len(cmdargs))

		for i, arg := range cmdargs {
			jobCmdArgs[i] = utils.Format(arg, cmd.Args.Data())
		}

		args.Set("args", jobCmdArgs)
		if len(env) > 0 {
			args.Set("env", env)
		}

		if workdir != "" {
			args.Set("working_dir", workdir)
		}

		extcmd := &core.Cmd{
			ID:   cmd.ID,
			Gid:  cmd.Gid,
			Nid:  cmd.Nid,
			Name: cmdExecute,
			Data: cmd.Data,
			Tags: cmd.Tags,
			Args: args,
		}

		return &extensionProcess{
			extps: NewExtProcess(extcmd),
			cmd:   cmd,
		}
	}

	return constructor
}

func (ps *extensionProcess) Cmd() *core.Cmd {
	return ps.cmd
}

func (ps *extensionProcess) Run() {
	//intercept all the messages from the 'execute' command and
	//change it to it's original value.

	//TODO: fix me! I will build but not work
	// var cfg RunCfg

	// extcfg := RunCfg{
	// 	ProcessManager: cfg.ProcessManager,
	// 	MeterHandler: func(cmd *core.Cmd, p *psutil.Process) {
	// 		cfg.MeterHandler(ps.cmd, p)
	// 	},
	// 	MessageHandler: func(msg *Message) {
	// 		msg.Cmd = ps.cmd
	// 		cfg.MessageHandler(msg)
	// 	},
	// 	ResultHandler: func(cmd *core.Cmd, result *core.JobResult) {
	// 		result.Args = ps.cmd.Args
	// 		result.Cmd = ps.cmd.Name
	// 		result.ID = ps.cmd.ID
	// 		result.Gid = ps.cmd.Gid
	// 		result.Nid = ps.cmd.Nid

	// 		cfg.ResultHandler(ps.cmd, result)
	// 	},
	// 	Signal: cfg.Signal,
	// }

	ps.extps.Run()
}

func (ps *extensionProcess) Kill() {
	ps.extps.Kill()
}

func (ps *extensionProcess) GetStats() *process.ProcessStats {
	return ps.extps.GetStats()
}

/*
RegisterCmd registers a new command (extension) so it can be executed via commands
*/
func RegisterCmd(cmd string, exe string, workdir string, cmdargs []string, env []string) {
	CmdMap[cmd] = newExtensionProcess(exe, workdir, cmdargs, env)
}

/*
UnregisterCmd removes an extension from the global registery
*/
func UnregisterCmd(cmd string) {
	delete(CmdMap, cmd)
}
