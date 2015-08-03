package builtin

import (
	"github.com/Jumpscale/jsagent/agent/lib/pm"
)

const (
	CMD_PING = "ping"
)

func init() {
	pm.CMD_MAP[CMD_PING] = InternalProcessFactory(ping)
}

func ping(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
	result := &pm.JobResult{
		Id:    cmd.Id,
		Gid:   cmd.Gid,
		Nid:   cmd.Nid,
		Args:  cmd.Args,
		State: "SUCCESS",
		Level: pm.L_RESULT_JSON,
		Data:  `"pong"`,
	}

	return result
}
