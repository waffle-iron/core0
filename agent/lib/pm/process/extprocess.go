package process

import (
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	psutils "github.com/shirou/gopsutil/process"
)

type extProcess struct {
	cmd      *core.Cmd
	ctrl     chan int
	pid      int
	runs     int
	process  *psutils.Process
	children []*psutils.Process
}
