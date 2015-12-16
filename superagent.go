package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Jumpscale/agent2/agent"
	"github.com/Jumpscale/agent2/agent/configuration"
	_ "github.com/Jumpscale/agent2/agent/lib/builtin"
	"github.com/Jumpscale/agent2/agent/lib/logger"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/pm/core"
	"github.com/Jumpscale/agent2/agent/lib/settings"
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

	config := settings.GetSettings(cfg)

	//build list with ACs that we will poll from.
	controllers := make(map[string]*agent.ControllerClient)
	for key, controllerCfg := range config.Controllers {
		controllers[key] = agent.NewControllerClient(&controllerCfg)
	}

	mgr := pm.NewPM(config.Main.MessageIDFile, config.Main.MaxJobs)

	//configure logging handlers from configurations
	logger.ConfigureLogging(mgr, controllers, config)

	//configure hubble functions from configurations
	agent.RegisterHubbleFunctions(controllers, config)

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

	if config.RedisStats.Enabled {
		redis := agent.NewRedisStatsBuffer(config.RedisStats.Address, "", 1000, time.Duration(config.RedisStats.FlushInterval)*time.Millisecond)
		mgr.AddStatsFlushHandler(redis.Handler)
	}

	//buffer stats massages and flush when one of the conditions is met (size of 1000 record or 120 sec passes from last
	//flush)
	statsBuffer := agent.NewACStatsBuffer(1000, 120*time.Second, controllers, config)
	mgr.AddStatsFlushHandler(statsBuffer.Handler)

	//handle process results. Forwards the result to the correct controller.
	mgr.AddResultHandler(func(cmd *core.Cmd, result *core.JobResult) {
		//send result to AC.
		//NOTE: we always force the real gid and nid on the result.
		result.Gid = config.Main.Gid
		result.Nid = config.Main.Nid

		res, _ := json.Marshal(result)
		controller, ok := controllers[result.Args.GetTag()]

		if !ok {
			//command isn't bind to any controller. This can be a startup command.
			log.Printf("Got orphan result: %s", res)
			return
		}

		url := controller.BuildURL(config.Main.Gid, config.Main.Nid, "result")

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
			Gid:  config.Main.Gid,
			Nid:  config.Main.Nid,
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
	configuration.WatchAndApply(mgr, config)

	//start jobs pollers.
	agent.StartPollers(mgr, controllers, config)

	//wait
	select {}
}
