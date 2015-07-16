package builtin

import (
    "github.com/Jumpscale/jsagent/agent/lib/pm"
    "encoding/json"
    "fmt"
)

const (
    CMD_GET_PROCESSES_STATS = "get_processes_stats"
)

func init() {
    pm.CMD_MAP[CMD_GET_PROCESSES_STATS] = InternalProcessFactory(getProcessesStats)
}

type GetStatsData struct {
    Domain string `json:domain`
    Name string `json:name`
}

func getProcessesStats(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
    result := pm.NewBasicJobResult(cmd)


    //load data
    data := GetStatsData{}
    json.Unmarshal([]byte(cmd.Data), &data)

    stats := make([]*pm.ProcessStats, 0, len(cfg.ProcessManager.Processes()))

    for _, process := range cfg.ProcessManager.Processes() {
        cmd := process.Cmd()

        if data.Domain != "" {
            if data.Domain != cmd.Args.GetString("domain") {
                continue
            }
        }

        if data.Name != "" {
            if data.Name != cmd.Args.GetString("name") {
                continue
            }
        }

        stats = append(stats, process.GetStats())
    }


    serialized, err := json.Marshal(stats)
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
