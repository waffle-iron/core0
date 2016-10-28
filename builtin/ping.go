package builtin

import (
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
)

const (
	cmdPing = "ping"
)

func init() {
	pm.CmdMap[cmdPing] = process.NewInternalProcessFactory(ping)
}

func ping(cmd *core.Cmd) (interface{}, error) {
	return "pong", nil
}
