package builtin

import (
	"encoding/json"
	"fmt"
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
	pm.CmdMap[cmdGetAggregatedStats] = InternalProcessFactory(getAggregatedStats)
}

func getAggregatedStats(cmd *core.Cmd, cfg pm.RunCfg) *core.JobResult {
	result := core.NewBasicJobResult(cmd)

	var stat process.ProcessStats

	for _, process := range cfg.ProcessManager.Processes() {
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
			log.Println(err)
		}
	} else {
		log.Println("Getting agent process error:", err)
	}

	serialized, err := json.Marshal(stat)
	if err != nil {
		result.State = pm.StateError
		result.Data = fmt.Sprintf("%v", err)
	} else {
		result.State = pm.StateSuccess
		result.Level = pm.LevelResultJSON
		result.Data = string(serialized)
	}

	return result
}
