package builtin

import (
	"encoding/json"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/shirou/gopsutil/host"
)

const (
	CmdGetOsInfo = "get_os_info"
)

func init() {
	pm.CmdMap[CmdGetOsInfo] = InternalProcessFactory(getOsInfo)
}

func getOsInfo(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)
	result.Level = pm.LevelResultJson

	info, err := host.HostInfo()

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
