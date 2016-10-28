package builtin

import (
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/pm/process"
	"syscall"
)

const (
	cmdReboot = "reboot"
)

func init() {
	pm.CmdMap[cmdReboot] = process.NewInternalProcessFactory(restart)
}

func restart(cmd *core.Cmd) (interface{}, error) {
	pm.GetManager().Killall()
	syscall.Sync()
	syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
	return nil, nil
}
