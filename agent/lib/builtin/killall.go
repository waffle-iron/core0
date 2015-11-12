package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/Jumpscale/agent2/agent/lib/pm/process"
)

const (
	cmdKillAll = "killall"
)

func init() {
	pm.CmdMap[cmdKillAll] = process.NewInternalProcessFactory(killall)
}

func killall(cmd *core.Cmd) (interface{}, error) {
	pm.GetManager().Killall()
	return true, nil
}
