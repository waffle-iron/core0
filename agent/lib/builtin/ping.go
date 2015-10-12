package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
)

const (
	CmdPing = "ping"
)

func init() {
	pm.CmdMap[CmdPing] = InternalProcessFactory(ping)
}

func ping(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)
	result.Level = pm.LevelResultJson
	result.State = pm.StateSuccess
	result.Data = `"pong"`

	return result
}
