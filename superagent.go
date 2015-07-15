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
    "flag"
    "os"
    "io/ioutil"
)

func main() {
    settings := agent.Settings{}
    var cfg string
    var help bool

    flag.BoolVar(&help, "h", false, "Print this help screen")
    flag.StringVar(&cfg, "c", "", "Path to config file")
    flag.Parse()

    printHelp := func() {
        fmt.Println("agent [options]")
        flag.PrintDefaults()
    }

    if help {
        printHelp()
        return
    }

    if cfg == "" {
        fmt.Println("Missing required option -c")
        flag.PrintDefaults()
        os.Exit(1)
    }

    utils.LoadTomlFile(cfg, &settings)

    //loading command history file
    //history file is used to remember long running jobs during reboots.
    var history []string
    hisstr, err := ioutil.ReadFile(settings.Main.HistoryFile)

    if err == nil {
        err = json.Unmarshal(hisstr, &history)
        if err != nil {
            log.Println("Failed to load history file, invalid syntax ", err)
            history = make([]string, 0)
        }
    } else {
        log.Println("Couldn't read history file")
        history = make([]string, 0)
    }

    //dump hisory file
    dumpHistory := func() {
        data, err := json.Marshal(history)
        if err != nil {
            log.Fatal("Failed to write history file")
        }

        ioutil.WriteFile(settings.Main.HistoryFile, data, 0644)
    }

    buildUrl := func (base string, endpoint string) string {
        base = strings.TrimRight(base, "/")
        return fmt.Sprintf("%s/%d/%d/%s", base,
            settings.Main.Gid,
            settings.Main.Nid,
            endpoint)
    }

    mgr := pm.NewPM(settings.Main.MessageIdFile)

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

    mgr.AddStatsdMeterHandler(func (statsd *stats.Statsd, cmd *pm.Cmd, ps *process.Process) {
        //for each long running external process this will be called every 2 sec
        //You can here collect all the data you want abou the process and feed
        //statsd.

        cpu, err := ps.CPUPercent(0)
        if err == nil {
            statsd.Gauage("cpu", fmt.Sprintf("%f", cpu))
        }

        mem, err := ps.MemoryInfo()
        if err == nil {
            statsd.Gauage("rss", fmt.Sprintf("%d", mem.RSS))
            statsd.Gauage("vms", fmt.Sprintf("%d", mem.VMS))
            statsd.Gauage("swap", fmt.Sprintf("%d", mem.Swap))
        }
    })

    mgr.AddStatsFlushHandler(func (stats *stats.Stats) {
        //This will be called per process per stats_interval seconds. with
        //all the aggregated stats for that process.
        res, _ := json.Marshal(stats)
        log.Println(string(res))
        for _, base := range settings.Main.AgentControllers {
            url := buildUrl(base, "stats")

            reader := bytes.NewBuffer(res)
            resp, err := http.Post(url, "application/json", reader)
            if err != nil {
                log.Println("Failed to send stats result to AC", url, err)
                return
            }
            defer resp.Body.Close()
        }
    })

    //build list with ACs that we will poll from.
    var controllers []string
    if len(settings.Channel.Cmds) > 0 {
        controllers = make([]string, len(settings.Channel.Cmds))
        for i := 0; i < len(settings.Channel.Cmds); i++ {
            controllers[i] = settings.Main.AgentControllers[settings.Channel.Cmds[i]]
        }
    } else {
        controllers = settings.Main.AgentControllers
    }

    //start pollers goroutines
    for aci, ac := range controllers {
        go func() {
            lastfail := time.Now().Unix()
            for {
                response, err := http.Get(buildUrl(ac, "cmd"))
                if err != nil {
                    log.Println("Failed to retrieve new commands from", ac, err)
                    if time.Now().Unix() - lastfail < 4 {
                        time.Sleep(4 * time.Second)
                    }
                    lastfail = time.Now().Unix()

                    continue
                }

                defer response.Body.Close()
                body, err := ioutil.ReadAll(response.Body)
                if err != nil {
                    log.Println("Failed to load response content", err)
                    continue
                }

                cmd, err := pm.LoadCmd(body)
                if err != nil {
                    log.Println("Failed to load cmd", err)
                    continue
                }

                //set command defaults
                //1 - stats_interval
                meterInt := cmd.Args.GetInt("stats_interval")
                if meterInt == 0 {
                    cmd.Args.Set("stats_interval", settings.Stats.Interval)
                }

                //tag command for routing.
                cmd.Args.SetTag(aci)
                log.Println("Starting command", cmd)

                if cmd.Args.GetInt("max_time") == -1 {
                    //that's a long running process.
                    history = append(history, string(body))
                    dumpHistory()
                }

                mgr.RunCmd(cmd)
            }
        } ()
    }

    //handle process results
    mgr.AddResultHandler(func (result *pm.JobResult) {
        //send result to AC.
        res, _ := json.Marshal(result)
        url := buildUrl(
            controllers[result.Args.GetTag()],
            "result")

        reader := bytes.NewBuffer(res)
        resp, err := http.Post(url, "application/json", reader)
        if err != nil {
            log.Println("Failed to send job result to AC", url, err)
            return
        }
        defer resp.Body.Close()
    })

    //register the execute commands
    for cmdKey, cmdCfg := range settings.Cmds {
        pm.RegisterCmd(cmdKey, cmdCfg.Binary, cmdCfg.Path, cmdCfg.Script)
    }

    //start process mgr.
    mgr.Run()

    //rerun history
    for i := 0; i < len(history); i ++ {
        cmd, err := pm.LoadCmd([]byte(history[i]))
        if err != nil {
            log.Println("Failed to load history command", history[i])
        }

        mgr.RunCmd(cmd)
    }

    //wait
    select {}
}
