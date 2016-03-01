package main

import (
	"fmt"
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/settings"
	"log"
)

//TODO: POC bootstrap. This will most probably get rewritten when the process is clearer

func registerExtensions(extensions map[string]settings.Extension) {
	for extKey, extCfg := range extensions {
		var env []string
		if len(extCfg.Env) > 0 {
			env = make([]string, 0, len(extCfg.Env))
			for ek, ev := range extCfg.Env {
				env = append(env, fmt.Sprintf("%v=%v", ek, ev))
			}
		}

		pm.RegisterCmd(extKey, extCfg.Binary, extCfg.Cwd, extCfg.Args, env)
	}
}

func startupServices(mgr *pm.PM, included *settings.IncludedSettings) {
	tree, errors := included.GetStartupTree()
	if len(errors) > 0 {
		for _, err := range errors {
			log.Println("ERROR: ", err)
		}
	}

	for _, startup := range tree.Services() {
		if startup.Args == nil {
			startup.Args = make(map[string]interface{})
		}

		cmd := &core.Cmd{
			Gid:  settings.Options.Gid(),
			Nid:  settings.Options.Nid(),
			ID:   startup.Key(),
			Name: startup.Name,
			Data: startup.Data,
			Args: core.NewMapArgs(startup.Args),
		}

		meterInt := cmd.Args.GetInt("stats_interval")
		if meterInt == 0 {
			cmd.Args.Set("stats_interval", settings.Settings.Stats.Interval)
		}

		log.Printf("Starting %s\n", cmd)
		mgr.RunCmd(cmd)
	}
}

//Bootstrap registers extensions and startup system services.
func Bootstrap(mgr *pm.PM) {
	included, errors := settings.Settings.GetIncludedSettings()
	if len(errors) > 0 {
		for _, err := range errors {
			log.Println("ERROR: ", err)
		}
	}

	//register core extensions
	registerExtensions(settings.Settings.Extensions)

	//register included extensions
	registerExtensions(settings.Settings.Extensions)

	//start up all services in correct order.
	startupServices(mgr, included)
}
