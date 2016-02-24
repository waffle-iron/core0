package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Jumpscale/agent8/agent"
	"github.com/Jumpscale/agent8/agent/configuration"
	_ "github.com/Jumpscale/agent8/agent/lib/builtin"
	"github.com/Jumpscale/agent8/agent/lib/logger"
	"github.com/Jumpscale/agent8/agent/lib/pm"
	"github.com/Jumpscale/agent8/agent/lib/pm/core"
	"github.com/Jumpscale/agent8/agent/lib/settings"
)

func getKeys(m map[string]*agent.ControllerClient) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	return keys
}

func main() {
	var cfg string
	var help bool
	var gid int
	var nid int

	flag.BoolVar(&help, "h", false, "Print this help screen")
	flag.StringVar(&cfg, "c", "", "Path to config file")
	flag.IntVar(&gid, "gid", 0, "Grid ID")
	flag.IntVar(&nid, "nid", 0, "Node ID")
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
	if gid == 0 || nid == 0 {
		fmt.Println("-gid and -nid are required options")
		flag.PrintDefaults()
		os.Exit(1)
	}

	config := settings.GetSettings(cfg)

	if errors := config.Validate(); len(errors) > 0 {
		for _, err := range errors {
			log.Println(err)
		}

		log.Fatal("\nConfig validation error, please fix and try again.")
	}

	//build list with ACs that we will poll from.
	controllers := make(map[string]*agent.ControllerClient)
	for key, controllerCfg := range config.Controllers {
		controllers[key] = agent.NewControllerClient(&controllerCfg)
	}

	mgr := pm.NewPM(config.Main.MessageIDFile, config.Main.MaxJobs)

	//configure logging handlers from configurations
	logger.ConfigureLogging(mgr, controllers, gid, nid, config)

	//configure hubble functions from configurations
	agent.RegisterHubbleFunctions(controllers, gid, nid, config)

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
		statsBuffer := agent.NewACStatsBuffer(1000, 120*time.Second, controllers, gid, nid, config)
		mgr.AddStatsFlushHandler(statsBuffer.Handler)
	}

	//handle process results. Forwards the result to the correct controller.
	mgr.AddResultHandler(func(cmd *core.Cmd, result *core.JobResult) {
		//send result to AC.
		//NOTE: we always force the real gid and nid on the result.
		result.Gid = gid
		result.Nid = nid

		res, _ := json.Marshal(result)
		controller, ok := controllers[result.Args.GetTag()]

		if !ok {
			//command isn't bind to any controller. This can be a startup command.
			log.Printf("Got orphan result: %s", res)
			return
		}

		url := controller.BuildURL(gid, nid, "result")

		reader := bytes.NewBuffer(res)
		resp, err := controller.Client.Post(url, "application/json", reader)
		if err != nil {
			log.Println("Failed to send job result to AC", url, err)
			return
		}
		resp.Body.Close()
	})

	//start process mgr.
	mgr.Run()

	//System is ready to receive commands.
	//before start polling on commands, lets run our startup commands
	//from config
	for id, startup := range config.Startup {
		if startup.Args == nil {
			startup.Args = make(map[string]interface{})
		}

		cmd := &core.Cmd{
			Gid:  gid,
			Nid:  nid,
			ID:   id,
			Name: startup.Name,
			Data: startup.Data,
			Args: core.NewMapArgs(startup.Args),
		}

		meterInt := cmd.Args.GetInt("stats_interval")
		if meterInt == 0 {
			cmd.Args.Set("stats_interval", config.Stats.Interval)
		}

		mgr.RunCmd(cmd)
	}

	//also register extensions and run startup commands from partial configuration files
	configuration.WatchAndApply(mgr, gid, nid, config)

	//start jobs pollers.
	agent.StartPollers(mgr, controllers, gid, nid, config)

	//wait
	select {}
}
