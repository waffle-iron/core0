package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
)

const (
	cmdKillAll = "killall"
)

func init() {
	pm.CmdMap[cmdKillAll] = InternalProcessFactory(killall)
}

func killall(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)

	cfg.ProcessManager.Killall()

	result.State = pm.StateSuccess

	return result
}
