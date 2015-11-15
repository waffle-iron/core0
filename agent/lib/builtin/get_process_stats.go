package builtin

import (
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/Jumpscale/agent2/agent/lib/pm/process"
)

const (
	cmdGetProcessStats = "get_process_stats"
)

func init() {
	pm.CmdMap[cmdGetProcessStats] = process.NewInternalProcessFactory(getProcessStats)
}

type getProcessStatsData struct {
	ID string `json:"id"`
}

func getProcessStats(cmd *core.Cmd) (interface{}, error) {
	//load data
	data := getProcessStatsData{}
	err := json.Unmarshal([]byte(cmd.Data), &data)
	if err != nil {
		return nil, err
	}

	runner, ok := pm.GetManager().Runners()[data.ID]

	if !ok {
		return nil, fmt.Errorf("Process with id '%s' doesn't exist", data.ID)
	}

	ps := runner.Process()
	if ps != nil {
		return ps.GetStats(), nil
	} else {
		return &process.ProcessStats{}, nil
	}

}
