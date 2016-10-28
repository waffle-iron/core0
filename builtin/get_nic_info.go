package builtin

import (
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
	"github.com/shirou/gopsutil/net"
)

const (
	cmdGetNicInfo = "info.nic"
)

func init() {
	pm.CmdMap[cmdGetNicInfo] = process.NewInternalProcessFactory(getNicInfo)
}

func getNicInfo(cmd *core.Command) (interface{}, error) {
	return net.Interfaces()
}
