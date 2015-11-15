package pm

//implement internal processes

import (
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/Jumpscale/agent2/agent/lib/pm/process"
)

/*
Global command ProcessConstructor registery
*/
var CmdMap = map[string]process.ProcessFactory{
	process.CommandExecute: process.NewSystemProcess,
}

/*
NewProcess creates a new process from a command
*/
func GetProcessFactory(cmd *core.Cmd) process.ProcessFactory {
	return CmdMap[cmd.Name]
}

/*
RegisterCmd registers a new command (extension) so it can be executed via commands
*/
func RegisterCmd(cmd string, exe string, workdir string, cmdargs []string, env []string) {
	CmdMap[cmd] = process.NewExtensionProcessFactory(exe, workdir, cmdargs, env)
}

/*
UnregisterCmd removes an extension from the global registery
*/
func UnregisterCmd(cmd string) {
	delete(CmdMap, cmd)
}
