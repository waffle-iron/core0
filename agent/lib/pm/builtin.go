package pm

//implement internal processes

import (
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"github.com/shirou/gopsutil/process"
)

const (
	CmdExecute = "execute"
)

type ProcessConstructor func(cmd *Cmd) Process

var CMD_MAP = map[string]ProcessConstructor{
	CmdExecute: NewExtProcess,
}

func NewProcess(cmd *Cmd) Process {
	constructor, ok := CMD_MAP[cmd.Name]
	if !ok {
		return nil
	}

	return constructor(cmd)
}

type ExtensionProcess struct {
	extps Process
	cmd   *Cmd
}

//Create a constructor for external process to execute an external script
//exe: name of executor, (python, lua, bash)
//workdir: working directory of script
//scriptname: if scriptname != "", execute that specific script, otherwise use args[name]
//  scriptname can have {<arg-key>} pattern that will be replaced with the value before execution
func extensionProcess(exe string, workdir string, cmdargs []string, env []string) ProcessConstructor {
	//create a new execute process with python2.7 or lua as executors.
	constructor := func(cmd *Cmd) Process {
		args := cmd.Args.Clone(false)
		args.Set("name", exe)

		job_cmdargs := make([]string, len(cmdargs))

		for i, arg := range cmdargs {
			job_cmdargs[i] = utils.Format(arg, cmd.Args.Data())
		}

		args.Set("args", job_cmdargs)
		if len(env) > 0 {
			args.Set("env", env)
		}

		if workdir != "" {
			args.Set("working_dir", workdir)
		}

		extcmd := &Cmd{
			Id:   cmd.Id,
			Gid:  cmd.Gid,
			Nid:  cmd.Nid,
			Name: CmdExecute,
			Data: cmd.Data,
			Args: args,
		}

		return &ExtensionProcess{
			extps: NewExtProcess(extcmd),
			cmd:   cmd,
		}
	}

	return constructor
}

func (ps *ExtensionProcess) Cmd() *Cmd {
	return ps.cmd
}

func (ps *ExtensionProcess) Run(cfg RunCfg) {
	//intercept all the messages from the 'execute' command and
	//change it to it's original value.
	extcfg := RunCfg{
		ProcessManager: cfg.ProcessManager,
		MeterHandler: func(cmd *Cmd, p *process.Process) {
			cfg.MeterHandler(ps.cmd, p)
		},
		MessageHandler: func(msg *Message) {
			msg.Cmd = ps.cmd
			cfg.MessageHandler(msg)
		},
		ResultHandler: func(result *JobResult) {
			result.Args = ps.cmd.Args
			result.Cmd = ps.cmd.Name
			result.Id = ps.cmd.Id
			result.Gid = ps.cmd.Gid
			result.Nid = ps.cmd.Nid

			cfg.ResultHandler(result)
		},
		Signal: cfg.Signal,
	}

	ps.extps.Run(extcfg)
}

func (ps *ExtensionProcess) Kill() {
	ps.extps.Kill()
}

func (ps *ExtensionProcess) GetStats() *ProcessStats {
	return ps.extps.GetStats()
}

//registers a command to the process manager.
//cmd: Command name
//exe: executing binary
//workdir: working directory
//script: script name
//if script == "", then script name will be used from cmd.Args.
func RegisterCmd(cmd string, exe string, workdir string, cmdargs []string, env []string) {
	CMD_MAP[cmd] = extensionProcess(exe, workdir, cmdargs, env)
}

func UnregisterCmd(cmd string) {
	delete(CMD_MAP, cmd)
}
