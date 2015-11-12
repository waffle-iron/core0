package builtin

import (
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
)

const (
	cmdGetProcessStats = "get_process_stats"
)

func init() {
	pm.CmdMap[cmdGetProcessStats] = InternalProcessFactory(getProcessStats)
}

type getProcessStatsData struct {
	ID string `json:"id"`
}

func getProcessStats(cmd *core.Cmd, cfg pm.RunCfg) *core.JobResult {
	result := core.NewBasicJobResult(cmd)

	//load data
	data := getProcessStatsData{}
	json.Unmarshal([]byte(cmd.Data), &data)

	process, ok := cfg.ProcessManager.Processes()[data.ID]

	if !ok {
		result.State = pm.StateError
		result.Data = fmt.Sprintf("Process with id '%s' doesn't exist", data.ID)
		return result
	}

	stats := process.GetStats()

	serialized, err := json.Marshal(stats)
	if err != nil {
		result.State = pm.StateError
		result.Data = fmt.Sprintf("%v", err)
	} else {
		result.State = pm.StateSuccess
		result.Level = pm.LevelResultJSON
		result.Data = string(serialized)
	}

	return result
}
