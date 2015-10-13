package builtin

import (
	"encoding/json"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/shirou/gopsutil/net"
)

const (
	cmdGetNicInfo = "get_nic_info"
)

func init() {
	pm.CmdMap[cmdGetNicInfo] = InternalProcessFactory(getNicInfo)
}

func getNicInfo(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)
	result.Level = pm.LevelResultJSON

	info, err := net.NetInterfaces()

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
