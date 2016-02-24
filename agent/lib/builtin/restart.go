package builtin

import (
	"github.com/Jumpscale/agent8/agent/lib/pm"
	"github.com/Jumpscale/agent8/agent/lib/pm/core"
	"github.com/Jumpscale/agent8/agent/lib/pm/process"
	"os"
)

const (
	cmdRestart = "restart"
)

func init() {
	pm.CmdMap[cmdRestart] = process.NewInternalProcessFactory(restart)
}

func restart(cmd *core.Cmd) (interface{}, error) {
	os.Exit(0)
	return nil, nil
}
