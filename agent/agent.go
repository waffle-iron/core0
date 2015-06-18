package main

import (
    "github.com/Jumpscale/jsagent/agent/lib/pm"
    "time"
    "log"
)

func main() {
    mgr := pm.NewPM()

    mgr.Run()

    args := &pm.BasicArgs{
        Name: "/bin/bash",
        CmdArgs: []string{"-c", "cat > test.txt"},
        //WorkingDir: "/home/azmy",
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
