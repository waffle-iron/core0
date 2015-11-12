package process

import (
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
)

//ProcessStats holds process cpu and memory usage
type ProcessStats struct {
	Cmd   *core.Cmd `json:"cmd"`
	CPU   float64   `json:"cpu"`
	RSS   uint64    `json:"rss"`
	VMS   uint64    `json:"vms"`
	Swap  uint64    `json:"swap"`
	Debug string    `json:"debug,ommitempty"`
}

//Process interface
type Process interface {
	Cmd() *core.Cmd
	Run()
	Kill()
	GetStats() *ProcessStats
}
