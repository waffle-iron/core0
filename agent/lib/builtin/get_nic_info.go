package builtin

import (
	"github.com/Jumpscale/agent8/agent/lib/pm"
	"github.com/Jumpscale/agent8/agent/lib/pm/core"
	"github.com/Jumpscale/agent8/agent/lib/pm/process"
	"github.com/shirou/gopsutil/net"
)

const (
	cmdGetNicInfo = "get_nic_info"
)

func init() {
	pm.CmdMap[cmdGetNicInfo] = process.NewInternalProcessFactory(getNicInfo)
}

func getNicInfo(cmd *core.Cmd) (interface{}, error) {
	return net.NetInterfaces()
}
