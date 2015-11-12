package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
)

const (
	cmdPing = "ping"
)

func init() {
	pm.CmdMap[cmdPing] = InternalProcessFactory(ping)
}

func ping(cmd *core.Cmd, cfg pm.RunCfg) *core.JobResult {
	result := core.NewBasicJobResult(cmd)
	result.Level = pm.LevelResultJSON
	result.State = pm.StateSuccess
	result.Data = `"pong"`

	return result
}
