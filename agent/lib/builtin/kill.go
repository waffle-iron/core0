package builtin

import (
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/pm"
)

const (
	cmdKill = "kill"
)

func init() {
	pm.CmdMap[cmdKill] = InternalProcessFactory(kill)
}

type killData struct {
	ID string `json:"id"`
}

func kill(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)

	//load data
	data := killData{}
	err := json.Unmarshal([]byte(cmd.Data), &data)

	if err != nil {
		result.State = pm.StateError
		result.Data = fmt.Sprintf("%v", err)
	} else {
		cfg.ProcessManager.Kill(data.ID)
		result.State = pm.StateSuccess
	}

	return result
}
