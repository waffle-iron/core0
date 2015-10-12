package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
)

const (
	CmdKillAll = "killall"
)

func init() {
	pm.CmdMap[CmdKillAll] = InternalProcessFactory(killall)
}

func killall(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)

	cfg.ProcessManager.Killall()

	result.State = pm.S_SUCCESS

	return result
}
