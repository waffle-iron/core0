package main

import (
	"time"

	"github.com/g8os/core.base"
	"github.com/g8os/core.base/pm"
	pmcore "github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/settings"
	"github.com/g8os/core0/bootstrap"
	_ "github.com/g8os/core0/builtin"
	"github.com/g8os/core0/logger"
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
			log.Errorf("Validation Error: %s\n", err)
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

	pm.InitProcessManager(config.Main.MaxJobs)

	//start process mgr.
	log.Infof("Starting process manager")
	mgr := pm.GetManager()
	mgr.Run()

	//start local transport
	log.Infof("Starting local transport")
	local, err := core.NewLocal("/var/run/core.sock")
	if err != nil {
		log.Errorf("Failed to start local transport: %s", err)
	} else {
		go local.Serve()
	}

	bs := bootstrap.NewBootstrap()
	bs.Bootstrap()

	//build list with ACs that we will poll from.
	controllers := make(map[string]*settings.ControllerClient)
	for key, controllerCfg := range config.Controllers {
		cl, err := controllerCfg.GetClient()
		if err != nil {
			log.Warning("Can't reach controller %s: %s", cl.URL, err)
		}

		controllers[key] = cl
	}

	//configure logging handlers from configurations
	log.Infof("Configure logging")
	logger.ConfigureLogging(controllers)

	log.Infof("Setting up stats buffers")
	if config.Stats.Redis.Enabled {
		redis := core.NewRedisStatsBuffer(config.Stats.Redis.Address, "", 1000, time.Duration(config.Stats.Redis.FlushInterval)*time.Millisecond)
		mgr.AddStatsFlushHandler(redis.Handler)
	}

	//handle process results. Forwards the result to the correct controller.
	mgr.AddResultHandler(func(cmd *pmcore.Command, result *pmcore.JobResult) {
		log.Infof("Job result for command '%s' is '%s': %v", cmd, result.State, result)
	})

	log.Infof("Configure and startup hubble agents")
	//start jobs sinks.
	core.StartSinks(pm.GetManager(), controllers)

	//wait
	select {}
}
