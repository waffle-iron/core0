package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"os"
)

const (
	cmdRestart = "restart"
)

func init() {
	pm.CmdMap[cmdRestart] = InternalProcessFactory(restart)
}

func restart(cmd *core.Cmd, cfg pm.RunCfg) *core.JobResult {
	os.Exit(0)
	return nil
}
