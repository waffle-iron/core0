package pm


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
    Name string `json:"name"`
    CmdArgs []string `json:"args"`
    MaxTime int `json:"max_time"`
    MaxRestart int `json:"max_restart"`
    Domain string `json:"domain"`
    WorkingDir string `json:"working_dir"`
    LogLevels []int `json:"loglevels"`
    LogLevelsDB []int `json:"logevels_db"`
    LogLevelsAC []int `json:"logevels_ac"`
    RecurringPeriod int `json:"recurring_period"`
    StatsInterval int `json:"stats_interval"`
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
    return args.LogLevels
}

func (args *BasicArgs) GetLogLevelsDB() []int {
    return args.LogLevelsDB
}

func (args *BasicArgs) GetLogLevelsAC() []int {
    return args.LogLevelsAC
}

func (args *BasicArgs) GetRecurringPeriod() int {
    return args.RecurringPeriod
}

func (args *BasicArgs) GetStatsInterval() int {
    return args.StatsInterval
}
