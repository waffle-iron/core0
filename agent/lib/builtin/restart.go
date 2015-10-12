package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"os"
)

const (
	CmdRestart = "restart"
)

func init() {
	pm.CMD_MAP[CmdRestart] = InternalProcessFactory(restart)
}

func restart(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	os.Exit(0)
	return nil
}
