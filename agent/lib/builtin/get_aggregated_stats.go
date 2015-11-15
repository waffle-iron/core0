package builtin

import (
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/Jumpscale/agent2/agent/lib/pm/process"
	psutil "github.com/shirou/gopsutil/process"
	"log"
	"os"
)

const (
	cmdGetAggregatedStats = "get_aggregated_stats"
)

func init() {
	pm.CmdMap[cmdGetAggregatedStats] = process.NewInternalProcessFactory(getAggregatedStats)
}

func getAggregatedStats(cmd *core.Cmd) (interface{}, error) {
	return nil, nil

	var stat process.ProcessStats

	for _, runner := range pm.GetManager().Runners() {
		process := runner.Process()
		if process == nil {
			continue
		}

		processStats := process.GetStats()
		stat.CPU += processStats.CPU
		stat.RSS += processStats.RSS
		stat.Swap += processStats.Swap
		stat.VMS += processStats.VMS
	}

	//also get agent cpu and memory consumption.
	if agent, err := psutil.NewProcess(int32(os.Getpid())); err == nil {
		agentCPU, err := agent.CPUPercent(0)
		if err == nil {
			stat.CPU += agentCPU
		} else {
			log.Println(err)
		}

		agentMem, err := agent.MemoryInfo()
		if err == nil {
			stat.RSS += agentMem.RSS
			stat.Swap += agentMem.Swap
			stat.VMS += agentMem.VMS
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}

	return stat, nil
}
