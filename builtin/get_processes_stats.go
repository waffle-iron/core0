package builtin

import (
	"encoding/json"
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
)

const (
	cmdGetProcessesStats = "get_processes_stats"
)

func init() {
	pm.CmdMap[cmdGetProcessesStats] = process.NewInternalProcessFactory(getProcessesStats)
}

type getStatsData struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
}

func getProcessesStats(cmd *core.Cmd) (interface{}, error) {
	//load data
	data := getStatsData{}
	err := json.Unmarshal([]byte(cmd.Data), &data)
	if err != nil {
		return nil, err
	}

	stats := make([]*process.ProcessStats, 0, len(pm.GetManager().Runners()))

	for _, runner := range pm.GetManager().Runners() {
		process := runner.Process()
		if process == nil {
			continue
		}

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

	return stats, nil
}
