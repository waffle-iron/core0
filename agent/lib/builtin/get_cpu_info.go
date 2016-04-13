package builtin

import (
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/pm/process"
	"github.com/shirou/gopsutil/cpu"
)

const (
	cmdGetCPUInfo = "get_cpu_info"
)

func init() {
	pm.CmdMap[cmdGetCPUInfo] = process.NewInternalProcessFactory(getCPUInfo)
}

func getCPUInfo(cmd *core.Cmd) (interface{}, error) {
	return cpu.Info()
}
