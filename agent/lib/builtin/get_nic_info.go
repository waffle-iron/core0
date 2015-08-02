package builtin

import (
    "github.com/Jumpscale/jsagent/agent/lib/pm"
    "github.com/shirou/gopsutil/net"
    "encoding/json"
)

const (
    CMD_GET_NIC_INFO = "get_nic_info"
)

func init() {
    pm.CMD_MAP[CMD_GET_NIC_INFO] = InternalProcessFactory(getNicInfo)
}

func getNicInfo(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
    result := &pm.JobResult {
        Id: cmd.Id,
        Gid: cmd.Gid,
        Nid: cmd.Nid,
        Args: cmd.Args,
        Level: pm.L_RESULT_JSON,
    }

    info, err := net.NetInterfaces()

    if err != nil {
        result.State = pm.S_ERROR
        m, _ := json.Marshal(err)
        result.Data = string(m)
    } else {
        result.State = pm.S_SUCCESS
        m, _ := json.Marshal(info)

        result.Data = string(m)
    }

    return result
}
