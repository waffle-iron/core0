package builtin

import (
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/pm/process"
	"github.com/shirou/gopsutil/disk"
)

const (
	cmdGetDiskInfo = "get_disk_info"
)

func init() {
	pm.CmdMap[cmdGetDiskInfo] = process.NewInternalProcessFactory(getDiskInfo)
}

func getDiskInfo(cmd *core.Cmd) (interface{}, error) {
	return disk.Partitions(true)
}