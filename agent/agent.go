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
        Name: "sleep",
        CmdArgs: []string{"30"},
        WorkingDir: "/home/azmy",
        //MaxTime: 10,
        MaxRestart: 2,
    }

    mgr.NewCmd("execute", "id", args, "")

    for {
        select {
        case <- time.After(10 * time.Second):
            log.Println("...")
        }
    }

}
