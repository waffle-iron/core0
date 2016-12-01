package builtin

import (
	"github.com/g8os/core0/base/pm"
	"github.com/g8os/core0/base/pm/core"
	"github.com/g8os/core0/base/pm/process"
	"github.com/shirou/gopsutil/cpu"
)

const (
	cmdGetCPUInfo = "info.cpu"
)

func init() {
	pm.CmdMap[cmdGetCPUInfo] = process.NewInternalProcessFactory(getCPUInfo)
}

func getCPUInfo(cmd *core.Command) (interface{}, error) {
	return cpu.Info()
}
