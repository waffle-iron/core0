package builtin

import (
    "time"
    "github.com/Jumpscale/jsagent/agent/lib/pm"
    "github.com/shirou/gopsutil/mem"
    "encoding/json"
)

const (
    CMD_GET_MEM_INFO = "get_mem_info"
)

func init() {
    pm.CMD_MAP[CMD_GET_MEM_INFO] = InternalProcessFactory(getMemInfo)
}

func getMemInfo(cmd *pm.Cmd, cfg pm.RunCfg) {
    result := &pm.JobResult {
        Id: cmd.Id,
        Gid: cmd.Gid,
        Nid: cmd.Nid,
        Args: cmd.Args,
        StartTime: time.Now().Unix(),
        Level: pm.L_RESULT_JSON,
    }

    defer func() {
        result.Time = time.Now().Unix() - result.StartTime
    } ()

    info, err := mem.VirtualMemory()

    if err != nil {
        result.State = pm.S_ERROR
        m, _ := json.Marshal(err)
        result.Data = string(m)

        return
    }

    result.State = pm.S_SUCCESS
    m, _ := json.Marshal(info)

    result.Data = string(m)

    cfg.ResultHandler(result)
}
