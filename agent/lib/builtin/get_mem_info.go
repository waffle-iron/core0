package builtin

import (
	"encoding/json"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/shirou/gopsutil/mem"
)

const (
	cmdGetMemInfo = "get_mem_info"
)

func init() {
	pm.CmdMap[cmdGetMemInfo] = InternalProcessFactory(getMemInfo)
}

func getMemInfo(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)
	result.Level = pm.LevelResultJson

	info, err := mem.VirtualMemory()

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
