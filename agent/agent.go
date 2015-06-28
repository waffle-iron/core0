package main

import (
    "github.com/Jumpscale/jsagent/agent/lib/pm"
    "github.com/Jumpscale/jsagent/agent/lib/logger"
    "github.com/Jumpscale/jsagent/agent/lib/stats"
    "github.com/shirou/gopsutil/process"
    "time"
    "encoding/json"
    "log"
    // "os"
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


    dblogger := logger.NewDBLogger(logger.NewSqliteFactory("./"))
    mgr.AddMessageHandler(dblogger.Log)

    aclogger := logger.NewACLogger("http://localhost:8080/log", 2, 10 * time.Second)
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
    // // `

    // margs := map[string]interface{} {
    //     "name": "python2.7",
    //     "args": []string{"test.py"},
    //     "loglevles": []int{3},
    //     "loglevels_db": []int{3},
    //     "max_time": 5,
    // }

    // args := pm.NewMapArgs(margs)

    cmd := map[string]interface{} {
        "id": "job-id",
        "gid": 1,
        "nid": 10,
        "name": "get_msgs",
        "args": map[string]interface{} {
            "loglevels": []int{1, 2, 3},
            "loglevels_db": []int{3},
            "max_time": 20,
        },
        "data": `{
            "idfrom": 0,
            "idto": 100,
            "timefrom": 100000,
            "timeto": 200000,
            "levels": "3-5"
        }`,
    }

    ping := map[string]interface{} {
        "id": "asdfasdg",
        "gid": 1,
        "nid": 10,
        "name": "ping",
        "args": map[string]interface{} {
            "loglevels": []int{1, 2, 3},
            "loglevels_db": []int{3},
            "max_time": 20,
            "recurring_period": 10,
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
            "max_restart": 2,
        },
        "data": "",
    }

    jscmd2 := map[string]interface{} {
        "id": "recurring",
        "gid": 1,
        "nid": 10,
        "name": "execute_js_py",
        "args": map[string]interface{} {
            "name": "recurring.py",
            "loglevels": []int{3},
            // "loglevels_db": []int{3},
            "max_time": 5,
            "recurring_period": 2,
        },
        "data": "",
    }

    killall := map[string]interface{} {
        "id": "kill",
        "gid": 1,
        "nid": 10,
        "name": "killall",
        "args": map[string]interface{} {

        },
    }

    mgr.NewMapCmd(cmd)
    mgr.NewMapCmd(ping)
    mgr.NewMapCmd(jscmd)
    mgr.NewMapCmd(jscmd2)

    // time.Sleep(3 * time.Second)
    mgr.NewMapCmd(killall)
    //mgr.NewMapCmd(restart)

    for {
        select {
        case <- time.After(10 * time.Second):
            log.Println("...")
        }
    }
}
