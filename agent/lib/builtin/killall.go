package builtin

import (
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/pm/process"
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
