package process

import (
	"github.com/Jumpscale/agent8/agent/lib/pm/core"
	"github.com/Jumpscale/agent8/agent/lib/pm/stream"
)

const (
	CommandExecute = "execute"
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
	Run() (<-chan *stream.Message, error)
	Kill()
	GetStats() *ProcessStats
}

type ProcessFactory func(*core.Cmd) Process
