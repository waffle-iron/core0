package builtin

import (
	"encoding/json"
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
)

const (
	cmdKill = "core.kill"
)

func init() {
	pm.CmdMap[cmdKill] = process.NewInternalProcessFactory(kill)
}

type killData struct {
	ID string `json:"id"`
}

func kill(cmd *core.Command) (interface{}, error) {
	//load data
	data := killData{}
	err := json.Unmarshal(cmd.Arguments, &data)

	if err != nil {
		return nil, err
	}

	pm.GetManager().Kill(data.ID)
	return true, nil
}
