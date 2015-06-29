package main

import (
    "github.com/Jumpscale/jsagent/agent"
    "github.com/Jumpscale/jsagent/agent/lib/pm"
    "github.com/Jumpscale/jsagent/agent/lib/logger"
    "github.com/Jumpscale/jsagent/agent/lib/stats"
    "github.com/Jumpscale/jsagent/agent/lib/utils"

    _ "github.com/Jumpscale/jsagent/agent/lib/builtin"
    "github.com/shirou/gopsutil/process"
    "time"
    "encoding/json"
    "log"
    "fmt"
    "strings"
)

func main() {
    settings := agent.Settings{}

    utils.LoadTomlFile("agent.toml", &settings)

    mgr := pm.NewPM(settings.Main.MessageIdFile)

    if settings.Stats.Interval == 0 {
        //set default flush interval of 5 min
        settings.Stats.Interval = 300
    }

    statsd := stats.NewStatsd(time.Duration(settings.Stats.Interval) * time.Second,
        func (key string, value float64) {
        //TODO: send values to ac
        log.Println("STATS", key, value)
    })

    //start statsd aggregation
    statsd.Run()


    //apply logging handlers.
    for _, logcfg := range settings.Logging {
        switch strings.ToLower(logcfg.Type) {
            case "db":
                sqlFactory := logger.NewSqliteFactory(logcfg.LogDir)
                handler := logger.NewDBLogger(sqlFactory, logcfg.Levels)
                mgr.AddMessageHandler(handler.Log)
            case "ac":
                var endpoints []string

                if len(logcfg.AgentControllers) > 0 {
                    //specific ones.
                    endpoints = make([]string, 0, len(logcfg.AgentControllers))
                    for _, aci := range logcfg.AgentControllers {
                        endpoints = append(endpoints, settings.Main.AgentControllers[aci])
                    }
                } else {
                    //all ACs
                    endpoints = make([]string, 0, len(settings.Main.AgentControllers))
                    for _, ac := range settings.Main.AgentControllers {
                        endpoints = append(endpoints, ac)
                    }
                }

                batchsize := 1000 // default
                flushint := 120 // default (in seconds)
                if logcfg.BatchSize != 0 {
                    batchsize = logcfg.BatchSize
                }
                if logcfg.FlushInt != 0 {
                    flushint = logcfg.FlushInt
                }

                handler := logger.NewACLogger(
                    endpoints,
                    batchsize,
                    time.Duration(flushint) * time.Second,
                    logcfg.Levels)
                mgr.AddMessageHandler(handler.Log)
            default:
                panic(fmt.Sprintf("Unsupported logger type: %s", logcfg.Type))
        }
    }

    mgr.AddMeterHandler(func (cmd *pm.Cmd, ps *process.Process) {
        //monitor.
        cpu, _ := ps.CPUPercent(0)
        statsd.Avg("cmd.cpu", cpu)
    })

    mgr.AddMessageHandler(func (msg *pm.Message) {
        log.Println(msg)
    })


    // dblogger := logger.NewDBLogger(logger.NewSqliteFactory("./"))
    // mgr.AddMessageHandler(dblogger.Log)

    // aclogger := logger.NewACLogger("http://localhost:8080/log", 2, 10 * time.Second)
    // mgr.AddMessageHandler(aclogger.Log)


    mgr.AddResultHandler(func (result *pm.JobResult) {
        s, _ := json.Marshal(result)
        log.Printf("%s", s)
    })

    //start process mgr.
    mgr.Run()

    // cmd := `
    // {
    //     "id": "job-id",
    //     "gid": "gid",
    //     "nid": "nid",
    //     "cmd": "execute",
    //     "args": {
    //         "name": "python2.7",
    //         "args": ["test.py"],
    //         "loglevles": [3],
    //         "loglevels_db": [3],
    //         "max_time": 5
    //     },
    //     "data": ""
    // }
    // // `

    // margs := map[string]interface{} {
    //     "name": "python2.7",
    //     "args": []string{"test.py"},
    //     "loglevles": []int{3},
    //     "loglevels_db": []int{3},
    //     "max_time": 5,
    // }

    // args := pm.NewMapArgs(margs)

    // cmd := map[string]interface{} {
    //     "id": "job-id",
    //     "gid": 1,
    //     "nid": 10,
    //     "name": "get_msgs",
    //     "args": map[string]interface{} {
    //         "loglevels": []int{1, 2, 3},
    //         "loglevels_db": []int{3},
    //         "max_time": 20,
    //     },
    //     "data": `{
    //         "idfrom": 0,
    //         "idto": 100,
    //         "timefrom": 100000,
    //         "timeto": 200000,
    //         "levels": "3-5"
    //     }`,
    // }

    mem := map[string]interface{} {
        "id": "asdfasdg",
        "gid": 1,
        "nid": 10,
        "name": "get_nic_info",
        "args": map[string]interface{} {
            "loglevels": []int{1, 2, 3},
            "loglevels_db": []int{3},
            "max_time": 20,
        },
    }

    // restart := map[string]interface{} {
    //     "id": "asdfasdg",
    //     "gid": 1,
    //     "nid": 10,
    //     "name": "restart",
    //     "args": map[string]interface{} {
    //         // "loglevels": []int{1, 2, 3},
    //         // "loglevels_db": []int{3},
    //         // "max_time": 20,
    //     },
    // }

    jscmd := map[string]interface{} {
        "id": "JS-job-id",
        "gid": 1,
        "nid": 10,
        "name": "execute_js_py",
        "args": map[string]interface{} {
            "name": "test.py",
            "loglevels": []int{3},
            "loglevels_db": []int{3},
            "max_time": 5,
            "recurring_period": 4,
            "max_restart": 2,
        },
        "data": "",
    }

    // jscmd2 := map[string]interface{} {
    //     "id": "recurring",
    //     "gid": 1,
    //     "nid": 10,
    //     "name": "execute_js_py",
    //     "args": map[string]interface{} {
    //         "name": "recurring.py",
    //         "loglevels": []int{3},
    //         // "loglevels_db": []int{3},
    //         "max_time": 5,
    //         "recurring_period": 2,
    //     },
    //     "data": "",
    // }

    // killall := map[string]interface{} {
    //     "id": "kill",
    //     "gid": 1,
    //     "nid": 10,
    //     "name": "killall",
    //     "args": map[string]interface{} {

    //     },
    // }

    mgr.NewMapCmd(mem)
    mgr.NewMapCmd(jscmd)
    for {
        select {
        case <- time.After(10 * time.Second):
            log.Println("...")
        }
    }
}
