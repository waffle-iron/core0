package pm

import (
    "github.com/Jumpscale/jsagent/agent/lib/utils"
)

type Args interface {
    GetName() string
    GetCmdArgs() []string
    GetMaxTime() int
    GetMaxRestart() int
    GetDomain() string
    GetWorkingDir() string
    GetLogLevels() []int
    GetLogLevelsDB() []int
    GetLogLevelsAC() []int
    GetRecurringPeriod() int
    GetStatsInterval() int
}

type BasicArgs struct {
    Name string
    CmdArgs []string
    MaxTime int
    MaxRestart int
    Domain string
    WorkingDir string
    LogLevels string
    LogLevelsDB string
    LogLevelsAC string
    RecurringPeriod int
    StatsInterval int
}

func (args *BasicArgs) GetName() string {
    return args.Name
}

func (args *BasicArgs) GetCmdArgs() []string {
    return args.CmdArgs
}

func (args *BasicArgs) GetMaxTime() int {
    return args.MaxTime
}

func (args *BasicArgs) GetMaxRestart() int {
    return args.MaxRestart
}

func (args *BasicArgs) GetDomain() string {
    return args.Domain
}

func (args *BasicArgs) GetWorkingDir() string {
    return args.WorkingDir
}

func (args *BasicArgs) GetLogLevels() []int {
    return utils.Expand(args.LogLevels, 1, 10)
}

func (args *BasicArgs) GetLogLevelsDB() []int {
    return utils.Expand(args.LogLevelsDB, 1, 10)
}

func (args *BasicArgs) GetLogLevelsAC() []int {
    return utils.Expand(args.LogLevelsAC, 1, 10)
}

func (args *BasicArgs) GetRecurringPeriod() int {
    return args.RecurringPeriod
}

func (args *BasicArgs) GetStatsInterval() int {
    return args.StatsInterval
}
