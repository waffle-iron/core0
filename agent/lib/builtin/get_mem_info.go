package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/Jumpscale/agent2/agent/lib/pm/process"
	"github.com/shirou/gopsutil/mem"
)

const (
	cmdGetMemInfo = "get_mem_info"
)

func init() {
	pm.CmdMap[cmdGetMemInfo] = process.NewInternalProcessFactory(getMemInfo)
}

func getMemInfo(cmd *core.Cmd) (interface{}, error) {
	return mem.VirtualMemory()
}
