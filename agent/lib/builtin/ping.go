package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
)

const (
	cmdPing = "ping"
)

func init() {
	pm.CmdMap[cmdPing] = InternalProcessFactory(ping)
}

func ping(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)
	result.Level = pm.LevelResultJSON
	result.State = pm.StateSuccess
	result.Data = `"pong"`

	return result
}
