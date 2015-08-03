package builtin

import (
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/jsagent/agent/lib/pm"
)

const (
	CMD_GET_PROCESS_STATS = "get_process_stats"
)

func init() {
	pm.CMD_MAP[CMD_GET_PROCESS_STATS] = InternalProcessFactory(getProcessStats)
}

type GetProcessStatsData struct {
	Id string `json:id`
}

func getProcessStats(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)

	//load data
	data := GetProcessStatsData{}
	json.Unmarshal([]byte(cmd.Data), &data)

	process, ok := cfg.ProcessManager.Processes()[data.Id]

	if !ok {
		result.State = pm.S_ERROR
		result.Data = fmt.Sprintf("Process with id '%s' doesn't exist", data.Id)
		return result
	}

	stats := process.GetStats()

	serialized, err := json.Marshal(stats)
	if err != nil {
		result.State = pm.S_ERROR
		result.Data = fmt.Sprintf("%v", err)
	} else {
		result.State = pm.S_SUCCESS
		result.Level = pm.L_RESULT_JSON
		result.Data = string(serialized)
	}

	return result
}
