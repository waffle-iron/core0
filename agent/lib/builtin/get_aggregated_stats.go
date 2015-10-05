package builtin

import (
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/shirou/gopsutil/process"
	"log"
	"os"
)

const (
	CMD_GET_AGGREGATED_STATS = "get_aggregated_stats"
)

func init() {
	pm.CMD_MAP[CMD_GET_AGGREGATED_STATS] = InternalProcessFactory(getAggregatedStats)
}

func getAggregatedStats(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := pm.NewBasicJobResult(cmd)

	var stat pm.ProcessStats

	for _, process := range cfg.ProcessManager.Processes() {
		processStats := process.GetStats()
		stat.CPU += processStats.CPU
		stat.RSS += processStats.RSS
		stat.Swap += processStats.Swap
		stat.VMS += processStats.VMS
	}

	//also get agent cpu and memory consumption.
	if agent, err := process.NewProcess(int32(os.Getpid())); err == nil {
		agentCpu, err := agent.CPUPercent(0)
		if err == nil {
			stat.CPU += agentCpu
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
		result.State = pm.S_ERROR
		result.Data = fmt.Sprintf("%v", err)
	} else {
		result.State = pm.S_SUCCESS
		result.Level = pm.L_RESULT_JSON
		result.Data = string(serialized)
	}

	return result
}
