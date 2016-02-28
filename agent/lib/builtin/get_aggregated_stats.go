package builtin

import (
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/pm/process"
	psutil "github.com/shirou/gopsutil/process"
	"log"
	"os"
)

const (
	cmdGetAggregatedStats = "get_aggregated_stats"
)

type aggregatedStatsMgr struct {
	agent *psutil.Process
}

func init() {
	agent, err := psutil.NewProcess(int32(os.Getpid()))
	if err != nil {
		log.Println("Failed to get referent to agent process", err)
	}

	mgr := &aggregatedStatsMgr{
		agent: agent,
	}

	pm.CmdMap[cmdGetAggregatedStats] = process.NewInternalProcessFactory(mgr.getAggregatedStats)
}

func (mgr *aggregatedStatsMgr) getAggregatedStats(cmd *core.Cmd) (interface{}, error) {
	stat := process.ProcessStats{}

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
	if mgr.agent != nil {
		agentCPU, err := mgr.agent.CPUPercent(0)
		if err == nil {
			stat.CPU += agentCPU
		} else {
			log.Println(err)
		}

		agentMem, err := mgr.agent.MemoryInfo()
		if err == nil {
			stat.RSS += agentMem.RSS
			stat.Swap += agentMem.Swap
			stat.VMS += agentMem.VMS
		} else {
			log.Println(err)
		}
	}

	return stat, nil
}
