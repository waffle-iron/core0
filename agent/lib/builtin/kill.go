package builtin

import (
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/jsagent/agent/lib/pm"
)

const (
	CMD_KILL = "kill"
)

func init() {
	pm.CMD_MAP[CMD_KILL] = InternalProcessFactory(kill)
}

type KillData struct {
	Id string `json:id`
}

func kill(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)

	//load data
	data := KillData{}
	err := json.Unmarshal([]byte(cmd.Data), &data)

	if err != nil {
		result.State = pm.S_ERROR
		result.Data = fmt.Sprintf("%v", err)
	} else {
		cfg.ProcessManager.Kill(data.Id)
		result.State = pm.S_SUCCESS
	}

	return result
}
