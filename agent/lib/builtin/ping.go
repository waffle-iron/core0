package builtin

import (
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/pm/process"
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
