package builtin

import (
	"github.com/Jumpscale/jsagent/agent/lib/pm"
)

const (
	CMD_KILLALL = "killall"
)

func init() {
	pm.CMD_MAP[CMD_KILLALL] = InternalProcessFactory(killall)
}

func killall(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)

	cfg.ProcessManager.Killall()

	result.State = pm.S_SUCCESS

	return result
}
