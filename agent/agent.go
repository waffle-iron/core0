package main

import (
    "github.com/Jumpscale/jsagent/agent/lib/pm"
    "github.com/Jumpscale/jsagent/agent/lib/stats"
    "github.com/shirou/gopsutil/process"
    "time"
    "log"
)

func main() {
    mgr := pm.NewPM()

    statsd := stats.NewStatsd(60, func (key string, value float64) {
        log.Println("STATS", key, value)
    })

    mgr.AddMeterHandler(func (cmd *pm.Cmd, ps *process.Process) {
        //monitor.
        cpu, _ := ps.CPUPercent(0)
        statsd.Avg("cmd.cpu", cpu)
    })

    statsd.Run()
    mgr.Run()

    // args := &pm.BasicArgs{
    //     Name: "/bin/bash",
    //     CmdArgs: []string{"-c", "cat > test.txt"},
    //     //WorkingDir: "/home/azmy",
    //     //MaxRestart: 2,
    //     //RecurringPeriod: 3,
    // }

    //nginx -c /etc/nginx/nginx.fg.conf
    args := &pm.BasicArgs{
        Name: "sudo",
        CmdArgs: []string{"nginx", "-c", "/etc/nginx/nginx.fg.conf"},
        WorkingDir: "/home/azmy",
        //MaxRestart: 2,
        //RecurringPeriod: 3,
    }

    mgr.NewCmd("execute", "id", args, "Hello world")

    for {
        select {
        case <- time.After(10 * time.Second):
            log.Println("...")
        }
    }

}
