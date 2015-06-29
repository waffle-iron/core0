package agent

type Logger struct {
    Type string
    AgentControllers []int
    Levels []int

    LogDir string
    FlushInt int
    BatchSize int
}


type Settings struct {
    Main struct {
        Gid int
        Nid int
        AgentControllers []string
        MessageIdFile string
    }

    PM struct {
        PyScriptsDir string
        LuaScriptsDir string
    }

    Logging map[string]Logger

    Stats struct {
        Interval int
        AgentControllers []int
    }

    Monitor struct {
        Interval int
    }

    Channel struct {
        Cmds []int
    }
}
