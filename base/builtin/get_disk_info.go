package builtin

import (
	"github.com/g8os/core0/base/pm"
	"github.com/g8os/core0/base/pm/core"
	"github.com/g8os/core0/base/pm/process"
	"github.com/shirou/gopsutil/disk"
)

const (
	cmdGetDiskInfo = "info.disk"
)

func init() {
	pm.CmdMap[cmdGetDiskInfo] = process.NewInternalProcessFactory(getDiskInfo)
}

func getDiskInfo(cmd *core.Command) (interface{}, error) {
	return disk.Partitions(true)
}
