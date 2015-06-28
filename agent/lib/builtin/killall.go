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

func killall(cmd *pm.Cmd, cfg pm.RunCfg) {
    cfg.ProcessManager.Killall()
}
