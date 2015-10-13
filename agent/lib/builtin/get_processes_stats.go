package builtin

import (
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/pm"
)

const (
	cmdGetProcessesStats = "get_processes_stats"
)

func init() {
	pm.CmdMap[cmdGetProcessesStats] = InternalProcessFactory(getProcessesStats)
}

type getStatsData struct {
	Domain string `json:domain`
	Name   string `json:name`
}

func getProcessesStats(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)

	//load data
	data := getStatsData{}
	json.Unmarshal([]byte(cmd.Data), &data)

	stats := make([]*pm.ProcessStats, 0, len(cfg.ProcessManager.Processes()))

	for _, process := range cfg.ProcessManager.Processes() {
		cmd := process.Cmd()

		if data.Domain != "" {
			if data.Domain != cmd.Args.GetString("domain") {
				continue
			}
		}

		if data.Name != "" {
			if data.Name != cmd.Args.GetString("name") {
				continue
			}
		}

		stats = append(stats, process.GetStats())
	}

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
