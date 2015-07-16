package builtin

import (
    "github.com/Jumpscale/jsagent/agent/lib/pm"
)

const (
    CMD_KILL = "kill"
)

func init() {
    pm.CMD_MAP[CMD_KILL] = InternalProcessFactory(kill)
}

type KillData struct {

}

func kill(cmd *pm.Cmd, cfg pm.RunCfg) {
    //result := pm.NewBasicJobResult(cmd)

    cfg.ProcessManager.Killall()

}
