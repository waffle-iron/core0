package builtin

import (
	"github.com/g8os/core0/base/pm"
	"github.com/g8os/core0/base/pm/core"
	"github.com/g8os/core0/base/pm/process"
)

const (
	cmdKillAll = "process.killall"
)

func init() {
	pm.CmdMap[cmdKillAll] = process.NewInternalProcessFactory(killall)
}

func killall(cmd *core.Command) (interface{}, error) {
	pm.GetManager().Killall()
	return true, nil
}
