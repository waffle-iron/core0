package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"os"
)

const (
	cmdRestart = "restart"
)

func init() {
	pm.CmdMap[cmdRestart] = InternalProcessFactory(restart)
}

func restart(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	os.Exit(0)
	return nil
}
