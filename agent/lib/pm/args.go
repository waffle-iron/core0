package pm

import (
    "encoding/json"
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

type MapArgs struct {
    data map[string]interface{}
}

func NewMapArgs(data map[string]interface{}) Args{
    return &MapArgs{
        data: data,
    }
}

func (args *MapArgs) MarshalJSON() ([]byte, error) {
    return json.Marshal(args.data)
}

func (args *MapArgs) getInt(key string) int {
    s, ok := args.data[key]
    if ok {
        return s.(int)
    }
    return 0
}

func (args *MapArgs) getString(key string) string {
    s, ok := args.data[key]
    if ok {
        return s.(string)
    }
    return ""
}

func (args *MapArgs) getFloat(key string) float64 {
    s, ok := args.data[key]
    if ok {
        return s.(float64)
    }
    return 0
}

func (args *MapArgs) getMap(key string) map[string]interface{} {
    s, ok := args.data[key]
    if ok {
        return s.(map[string]interface{})
    }

    return make(map[string]interface{})
}

func (args *MapArgs) getArray(key string) []interface{} {
    s, ok := args.data[key]
    if ok {
        return s.([]interface{})
    }

    return make([]interface{}, 0)
}

func (args *MapArgs) getStringArray(key string) []string {
    s, ok := args.data[key]
    if ok {
        return s.([]string)
    }

    return make([]string, 0)
}

func (args *MapArgs) getIntArray(key string) []int {
    s, ok := args.data[key]
    if ok {
        return s.([]int)
    }

    return make([]int, 0)
}

func (args *MapArgs) GetName() string {
    return args.getString("name")
}

func (args *MapArgs) GetCmdArgs() []string {
    return args.getStringArray("args")
}

func (args *MapArgs) GetMaxTime() int {
    return args.getInt("max_time")
}

func (args *MapArgs) GetMaxRestart() int {
    return args.getInt("max_restart")
}

func (args *MapArgs) GetDomain() string {
    return args.getString("domain")
}

func (args *MapArgs) GetWorkingDir() string {
    return args.getString("working_dir")
}

func (args *MapArgs) GetLogLevels() []int {
    return args.getIntArray("loglevels")
}

func (args *MapArgs) GetLogLevelsDB() []int {
    return args.getIntArray("logevels_db")
}

func (args *MapArgs) GetLogLevelsAC() []int {
    return args.getIntArray("logevels_ac")
}

func (args *MapArgs) GetRecurringPeriod() int {
    return args.getInt("recurring_period")
}

func (args *MapArgs) GetStatsInterval() int {
    return args.getInt("stats_interval")
}
