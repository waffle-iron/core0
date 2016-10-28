package builtin

import (
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
	"github.com/shirou/gopsutil/host"
)

const (
	cmdGetOsInfo = "get_os_info"
)

func init() {
	pm.CmdMap[cmdGetOsInfo] = process.NewInternalProcessFactory(getOsInfo)
}

func getOsInfo(cmd *core.Cmd) (interface{}, error) {
	return host.Info()
}
