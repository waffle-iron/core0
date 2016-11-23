package builtin

import (
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
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
