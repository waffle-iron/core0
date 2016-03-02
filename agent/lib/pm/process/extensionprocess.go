package process

import (
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/pm/stream"
	"github.com/g8os/core/agent/lib/utils"
)

type extensionProcess struct {
	system Process
	cmd    *core.Cmd
}

func NewExtensionProcessFactory(exe string, workdir string, cmdargs []string, env []string) ProcessFactory {

	constructor := func(table PIDTable, cmd *core.Cmd) Process {
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
			Name: CommandExecute,
			Data: cmd.Data,
			Tags: cmd.Tags,
			Args: args,
		}

		return &extensionProcess{
			system: NewSystemProcess(table, extcmd),
			cmd:    cmd,
		}
	}

	return constructor
}

func (process *extensionProcess) Cmd() *core.Cmd {
	return process.cmd
}

func (process *extensionProcess) Run() (<-chan *stream.Message, error) {
	return process.system.Run()
}

func (process *extensionProcess) Kill() {
	process.system.Kill()
}

func (process *extensionProcess) GetStats() *ProcessStats {
	return process.system.GetStats()
}
