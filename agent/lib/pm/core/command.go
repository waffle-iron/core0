package core

import (
	"encoding/json"
	"fmt"
)

//Cmd is an executable command
type Cmd struct {
	ID    string   `json:"id"`
	Gid   int      `json:"gid"`
	Nid   int      `json:"nid"`
	Roles []string `json:"roles"`
	Name  string   `json:"cmd"`
	Args  *MapArgs `json:"args"`
	Data  string   `json:"data"`
	Tags  string   `json:"tags"`
}

//String represents cmd as a string
func (cmd *Cmd) String() string {
	return fmt.Sprintf("(%s# %s %s)", cmd.ID, cmd.Name, cmd.Args.GetString("name"))
}

//NewMapCmd builds a cmd from a map.
func NewMapCmd(data map[string]interface{}) *Cmd {
	stdin, ok := data["data"]
	if !ok {
		stdin = ""
	}
	cmd := &Cmd{
		Gid:  data["gid"].(int),
		Nid:  data["nid"].(int),
		ID:   data["id"].(string),
		Name: data["name"].(string),
		Data: stdin.(string),
		Args: NewMapArgs(data["args"].(map[string]interface{})),
	}

	return cmd
}

//LoadCmd loads cmd from json string.
func LoadCmd(str []byte) (*Cmd, error) {
	var cmd Cmd
	err := json.Unmarshal(str, &cmd)
	if err != nil {
		return nil, err
	}
	if cmd.Args == nil || cmd.Args.Data() == nil {
		cmd.Args = NewMapArgs(map[string]interface{}{})
	}

	return &cmd, err
}
