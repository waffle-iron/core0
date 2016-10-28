package builtin

import (
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
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
