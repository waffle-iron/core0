package builtin

import (
	"github.com/g8os/core0/base/pm"
	"github.com/g8os/core0/base/pm/core"
	"github.com/g8os/core0/base/pm/process"
	"github.com/shirou/gopsutil/host"
)

const (
	cmdGetOsInfo = "info.os"
)

func init() {
	pm.CmdMap[cmdGetOsInfo] = process.NewInternalProcessFactory(getOsInfo)
}

func getOsInfo(cmd *core.Command) (interface{}, error) {
	return host.Info()
}
