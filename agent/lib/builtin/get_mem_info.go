package builtin

import (
	"encoding/json"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/shirou/gopsutil/mem"
)

const (
	CmdGetMemInfo = "get_mem_info"
)

func init() {
	pm.CmdMap[CmdGetMemInfo] = InternalProcessFactory(getMemInfo)
}

func getMemInfo(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)
	result.Level = pm.L_RESULT_JSON

	info, err := mem.VirtualMemory()

	if err != nil {
		result.State = pm.S_ERROR
		m, _ := json.Marshal(err)
		result.Data = string(m)
	} else {
		result.State = pm.S_SUCCESS
		m, _ := json.Marshal(info)

		result.Data = string(m)
	}

	return result
}
