package builtin

import (
	"encoding/json"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/shirou/gopsutil/cpu"
)

const (
	cmdGetCPUInfo = "get_cpu_info"
)

func init() {
	pm.CmdMap[cmdGetCPUInfo] = InternalProcessFactory(getCPUInfo)
}

func getCPUInfo(cmd *core.Cmd, cfg pm.RunCfg) *core.JobResult {
	result := pm.NewBasicJobResult(cmd)
	result.Level = pm.LevelResultJSON

	info, err := cpu.CPUInfo()

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
