package main

import (
	"fmt"
	"github.com/g8os/core/agent/lib/pm"
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

	mgr.RunSlice(tree.Slice(0, -1))
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
