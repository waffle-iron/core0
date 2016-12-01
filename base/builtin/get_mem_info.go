package builtin

import (
	"github.com/g8os/core0/base/pm"
	"github.com/g8os/core0/base/pm/core"
	"github.com/g8os/core0/base/pm/process"
	"github.com/shirou/gopsutil/mem"
)

const (
	cmdGetMemInfo = "info.mem"
)

func init() {
	pm.CmdMap[cmdGetMemInfo] = process.NewInternalProcessFactory(getMemInfo)
}

func getMemInfo(cmd *core.Command) (interface{}, error) {
	return mem.VirtualMemory()
}
