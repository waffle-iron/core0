package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
)

const (
	cmdKillAll = "killall"
)

func init() {
	pm.CmdMap[cmdKillAll] = InternalProcessFactory(killall)
}

func killall(cmd *core.Cmd, cfg pm.RunCfg) *core.JobResult {
	result := core.NewBasicJobResult(cmd)

	cfg.ProcessManager.Killall()

	result.State = pm.StateSuccess

	return result
}
