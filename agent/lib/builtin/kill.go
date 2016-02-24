package builtin

import (
	"encoding/json"
	"github.com/Jumpscale/agent8/agent/lib/pm"
	"github.com/Jumpscale/agent8/agent/lib/pm/core"
	"github.com/Jumpscale/agent8/agent/lib/pm/process"
)

const (
	cmdKill = "kill"
)

func init() {
	pm.CmdMap[cmdKill] = process.NewInternalProcessFactory(kill)
}

type killData struct {
	ID string `json:"id"`
}

func kill(cmd *core.Cmd) (interface{}, error) {
	//load data
	data := killData{}
	err := json.Unmarshal([]byte(cmd.Data), &data)

	if err != nil {
		return nil, err
	}

	pm.GetManager().Kill(data.ID)
	return true, nil
}
