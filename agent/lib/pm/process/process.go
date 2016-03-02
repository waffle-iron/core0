package process

import (
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/pm/stream"
	"syscall"
)

const (
	CommandExecute = "execute"
)

type GetPID func() (int, error)

type PIDTable interface {
	//Register atomic registration of PID. MUST grantee that that no wait4 will happen
	//on any of the child process until the register operation is done.
	Register(g GetPID) error
	Wait(pid int) *syscall.WaitStatus
}

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

type ProcessFactory func(PIDTable, *core.Cmd) Process
