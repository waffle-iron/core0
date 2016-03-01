package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/g8os/core/agent"
	_ "github.com/g8os/core/agent/lib/builtin"
	"github.com/g8os/core/agent/lib/logger"
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/settings"
	"github.com/g8os/core/agent/lib/system"
	"os"
)

func main() {
	if errors := settings.Options.Validate(); len(errors) != 0 {
		for _, err := range errors {
			fmt.Printf("Validation Error: %s\n", err)
		}

		os.Exit(1)
	}

	var options = settings.Options

	if err := settings.LoadSettings(options.Config()); err != nil {
		log.Fatal(err)
	}

	if errors := settings.Settings.Validate(); len(errors) > 0 {
		for _, err := range errors {
			log.Println(err)
		}

		log.Fatal("\nConfig validation error, please fix and try again.")
	}

	var config = settings.Settings

	//build list with ACs that we will poll from.
	controllers := make(map[string]*agent.ControllerClient)
	for key, controllerCfg := range config.Controllers {
		controllers[key] = agent.NewControllerClient(&controllerCfg)
	}

	mgr := pm.NewPM(config.Main.MessageIDFile, config.Main.MaxJobs)

	//configure logging handlers from configurations
	logger.ConfigureLogging(mgr, controllers)

	//configure hubble functions from configurations
	agent.RegisterHubbleFunctions(controllers)

	//register the extensions from the main configuration
	for extKey, extCfg := range config.Extensions {
		var env []string
		if len(extCfg.Env) > 0 {
			env = make([]string, 0, len(extCfg.Env))
			for ek, ev := range extCfg.Env {
				env = append(env, fmt.Sprintf("%v=%v", ek, ev))
			}
		}

		pm.RegisterCmd(extKey, extCfg.Binary, extCfg.Cwd, extCfg.Args, env)
	}

	if config.Stats.Redis.Enabled {
		redis := agent.NewRedisStatsBuffer(config.Stats.Redis.Address, "", 1000, time.Duration(config.Stats.Redis.FlushInterval)*time.Millisecond)
		mgr.AddStatsFlushHandler(redis.Handler)
	}

	if config.Stats.Ac.Enabled {
		//buffer stats massages and flush when one of the conditions is met (size of 1000 record or 120 sec passes from last
		//flush)
		statsBuffer := agent.NewACStatsBuffer(1000, 120*time.Second, controllers)
		mgr.AddStatsFlushHandler(statsBuffer.Handler)
	}

	//handle process results. Forwards the result to the correct controller.
	mgr.AddResultHandler(func(cmd *core.Cmd, result *core.JobResult) {
		//send result to AC.
		//NOTE: we always force the real gid and nid on the result.
		result.Gid = options.Gid()
		result.Nid = options.Nid()

		res, _ := json.Marshal(result)
		controller, ok := controllers[result.Args.GetTag()]

		if !ok {
			//command isn't bind to any controller. This can be a startup command.
			log.Printf("Got orphan result: %s", res)
			return
		}

		url := controller.BuildURL("result")

		reader := bytes.NewBuffer(res)
		resp, err := controller.Client.Post(url, "application/json", reader)
		if err != nil {
			log.Println("Failed to send job result to AC", url, err)
			return
		}
		resp.Body.Close()
	})

	//start the child processes cleaner
	system.CollectDefunct()

	//start process mgr.
	mgr.Run()

	////System is ready to receive commands.
	////before start polling on commands, lets run our startup commands
	////from config
	//for id, startup := range config.Startup {
	//	if startup.Args == nil {
	//		startup.Args = make(map[string]interface{})
	//	}
	//
	//	cmd := &core.Cmd{
	//		Gid:  options.Gid(),
	//		Nid:  options.Nid(),
	//		ID:   id,
	//		Name: startup.Name,
	//		Data: startup.Data,
	//		Args: core.NewMapArgs(startup.Args),
	//	}
	//
	//	meterInt := cmd.Args.GetInt("stats_interval")
	//	if meterInt == 0 {
	//		cmd.Args.Set("stats_interval", config.Stats.Interval)
	//	}
	//
	//	log.Printf("Starting %s\n", cmd)
	//	mgr.RunCmd(cmd)
	//}

	//start jobs pollers.
	agent.StartPollers(mgr, controllers)

	//wait
	select {}
}
