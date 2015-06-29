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
    "net/http"
    "log"
    "fmt"
    "strings"
    "bytes"
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

    buildUrl := func (base string, endpoint string) string {
        base = strings.TrimRight(base, "/")
        return fmt.Sprintf("%s/%d/%d/%s", base,
            settings.Main.Gid,
            settings.Main.Nid,
            endpoint)
    }

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
                        endpoints = append(
                            endpoints,
                            buildUrl(settings.Main.AgentControllers[aci], "log"))
                    }
                } else {
                    //all ACs
                    endpoints = make([]string, 0, len(settings.Main.AgentControllers))
                    for _, ac := range settings.Main.AgentControllers {
                        endpoints = append(
                            endpoints,
                            buildUrl(ac, "log"))
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
            case "console":
                handler := logger.NewConsoleLogger(logcfg.Levels)
                mgr.AddMessageHandler(handler.Log)
            default:
                panic(fmt.Sprintf("Unsupported logger type: %s", logcfg.Type))
        }
    }

    mgr.AddResultHandler(func (result *pm.JobResult) {
        //send result to AC.
        res, _ := json.Marshal(result)
        log.Println(string(res))
        url := buildUrl(
            settings.Main.AgentControllers[result.Args.GetTag()],
            "result")

        reader := bytes.NewBuffer(res)
        resp, err := http.Post(url, "application/json", reader)
        if err != nil {
            log.Println("Failed to send job result to AC", url, err)
            return
        }
        defer resp.Body.Close()
    })


    mgr.AddMeterHandler(func (cmd *pm.Cmd, ps *process.Process) {
        //monitor.
        cpu, _ := ps.CPUPercent(0)
        statsd.Avg("cmd.cpu", cpu)
    })

    //start process mgr.
    mgr.Run()

    //example command.
    scmd := `
    {
        "id": "job-id",
        "gid": 0,
        "nid": 1,
        "cmd": "execute_js_py",
        "args": {
            "name": "test.py",
            "loglevels": [3],
            "loglevels_db": [3],
            "max_time": 5
        }
    }
    `

    cmd, err := pm.LoadCmd([]byte(scmd))
    if err != nil {
        log.Fatal(err)
    }

    mgr.RunCmd(cmd)
    for {
        select {
        case <- time.After(10 * time.Second):
            log.Println("...")
        }
    }
}
