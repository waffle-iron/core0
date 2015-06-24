package main

import (
    "github.com/Jumpscale/jsagent/agent/lib/pm"
    "github.com/Jumpscale/jsagent/agent/lib/stats"
    "github.com/shirou/gopsutil/process"
    "time"
    "encoding/json"
    "log"
)

func main() {
    mgr := pm.NewPM("./mid.f")

    statsd := stats.NewStatsd(60, func (key string, value float64) {
        log.Println("STATS", key, value)
    })

    //start statsd aggregation
    statsd.Run()

    mgr.AddMeterHandler(func (cmd *pm.Cmd, ps *process.Process) {
        //monitor.
        cpu, _ := ps.CPUPercent(0)
        statsd.Avg("cmd.cpu", cpu)
    })

    mgr.AddMessageHandler(func (msg *pm.Message) {
        log.Println(msg)
    })

    dblogger := pm.NewDBLogger(pm.NewSqliteFactory("./"))
    mgr.AddMessageHandler(dblogger.Log)

    aclogger := pm.NewACLogger("http://localhost:8080/log", 2, 10 * time.Second)
    mgr.AddMessageHandler(aclogger.Log)


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
    // `

    margs := map[string]interface{} {
        "name": "python2.7",
        "args": []string{"test.py"},
        "loglevles": []int{3},
        "loglevels_db": []int{3},
        "max_time": 5,
    }

    args := pm.NewMapArgs(margs)

    // args := &pm.BasicArgs{
    //     Name: "python2.7",
    //     CmdArgs: []string{"test.py"},
    //     LogLevels: []int{3},
    //     LogLevelsDB: []int{3},
    //     MaxTime: 5,
    //     //WorkingDir: "/home/azmy",
    //     //MaxRestart: 2,
    //     //RecurringPeriod: 3,
    // }

    //nginx -c /etc/nginx/nginx.fg.conf
    // args := &pm.BasicArgs{
    //     Name: "sudo",
    //     CmdArgs: []string{"nginx", "-c", "/etc/nginx/nginx.fg.conf"},
    //     WorkingDir: "/home/azmy",
    //     LogLevels: []int {1, 2},
    //     LogLevelsDB: []int {2},
    //     LogLevelsAC: []int {2},
    //     //MaxRestart: 2,
    //     //RecurringPeriod: 3,
    // }

    mgr.NewCmd("execute", "id", args, "Hello world")

    for {
        select {
        case <- time.After(10 * time.Second):
            log.Println("...")
        }
    }
}
