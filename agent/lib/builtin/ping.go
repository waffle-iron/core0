package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
)

const (
	CMD_PING = "ping"
)

func init() {
	pm.CMD_MAP[CMD_PING] = InternalProcessFactory(ping)
}

func ping(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)
	result.Level = pm.L_RESULT_JSON
	result.State = pm.S_SUCCESS
	result.Data = `"pong"`

	return result
}
