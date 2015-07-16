package builtin

import (
    "os"
    "github.com/Jumpscale/jsagent/agent/lib/pm"
)

const (
    CMD_RESTART = "restart"
)

func init() {
    pm.CMD_MAP[CMD_RESTART] = InternalProcessFactory(restart)
}

func restart(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
    os.Exit(0)
    return nil
}
