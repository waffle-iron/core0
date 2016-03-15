package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/g8os/core/agent"
	_ "github.com/g8os/core/agent/lib/builtin"
	"github.com/g8os/core/agent/lib/logger"
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/settings"
	"github.com/op/go-logging"
	"os"
)

var (
	log = logging.MustGetLogger("main")
)

func init() {
	formatter := logging.MustStringFormatter("%{color}%{module} %{level:.1s} > %{message} %{color:reset}")
	logging.SetFormatter(formatter)
}

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
			log.Errorf("%s", err)
		}

		log.Fatalf("\nConfig validation error, please fix and try again.")
	}

	if settings.Settings.Controllers == nil {
		settings.Settings.Controllers = make(map[string]settings.Controller)
	}

	var config = settings.Settings

	pm.InitProcessManager(config.Main.MessageIDFile, config.Main.MaxJobs)

	//start process mgr.
	log.Infof("Starting process manager")
	mgr := pm.GetManager()
	mgr.Run()

	bootstrap := agent.NewBootstrap()
	bootstrap.Bootstrap()

	//build list with ACs that we will poll from.
	controllers := make(map[string]*settings.ControllerClient)
	for key, controllerCfg := range config.Controllers {
		controllers[key] = controllerCfg.GetClient()
	}

	//configure logging handlers from configurations
	log.Infof("Configure logging")
	logger.ConfigureLogging(controllers)

	log.Infof("Setting up stats buffers")
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
			if result.State != core.StateSuccess {
				log.Warningf("Got orphan result: %s", res)
			}

			return
		}

		url := controller.BuildURL("result")

		reader := bytes.NewBuffer(res)
		resp, err := controller.Client.Post(url, "application/json", reader)
		if err != nil {
			log.Errorf("Failed to send job result to controller '%s': %s", url, err)
			return
		}
		resp.Body.Close()
	})

	log.Infof("Configure and startup hubble agents")
	//configure hubble functions from configurations
	agent.RegisterHubbleFunctions(controllers)

	//start jobs pollers.
	agent.StartPollers(controllers)

	//wait
	select {}
}
