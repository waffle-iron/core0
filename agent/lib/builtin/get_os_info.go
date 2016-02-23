package builtin

import (
	"github.com/Jumpscale/agent8/agent/lib/pm"
	"github.com/Jumpscale/agent8/agent/lib/pm/core"
	"github.com/Jumpscale/agent8/agent/lib/pm/process"
	"github.com/shirou/gopsutil/host"
)

const (
	cmdGetOsInfo = "get_os_info"
)

func init() {
	pm.CmdMap[cmdGetOsInfo] = process.NewInternalProcessFactory(getOsInfo)
}

func getOsInfo(cmd *core.Cmd) (interface{}, error) {
	return host.HostInfo()
}
