package core

import (
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
