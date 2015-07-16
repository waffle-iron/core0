package builtin

import (
    "time"
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
    result.StartTime = time.Now().Unix()

    cfg.ProcessManager.Killall()

    result.State = pm.S_SUCCESS

    return result
}
