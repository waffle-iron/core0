package builtin

import (
	"encoding/json"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/shirou/gopsutil/disk"
)

const (
	cmdGetDiskInfo = "get_disk_info"
)

func init() {
	pm.CmdMap[cmdGetDiskInfo] = InternalProcessFactory(getDiskInfo)
}

func getDiskInfo(cmd *core.Cmd, cfg pm.RunCfg) *core.JobResult {
	result := core.NewBasicJobResult(cmd)
	result.Level = pm.LevelResultJSON

	info, err := disk.DiskPartitions(true)

	if err != nil {
		result.State = pm.StateError
		m, _ := json.Marshal(err)
		result.Data = string(m)
	} else {
		result.State = pm.StateSuccess
		m, _ := json.Marshal(info)

		result.Data = string(m)
	}

	return result
}
