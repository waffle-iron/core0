package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Jumpscale/agent2/agent"
	"github.com/Jumpscale/agent2/agent/configuration"
	_ "github.com/Jumpscale/agent2/agent/lib/builtin"
	"github.com/Jumpscale/agent2/agent/lib/logger"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/settings"
)

const (
	RECONNECT_SLEEP = 4
)

func getKeys(m map[string]*agent.ControllerClient) []string {
	keys := make([]string, 0, len(m))
	for key, _ := range m {
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
		controllers[key] = agent.NewControllerClient(controllerCfg)
	}

	mgr := pm.NewPM(config.Main.MessageIdFile, config.Main.MaxJobs)

	logger.ConfigureLogging(mgr, controllers, config)

	//buffer stats massages and flush when one of the conditions is met (size of 1000 record or 120 sec passes from last
	//flush)
	statsBuffer := agent.NewStatsBuffer(1000, 120*time.Second, controllers, config)
	mgr.AddStatsFlushHandler(statsBuffer.Handler)

	//handle process results
	mgr.AddResultHandler(func(result *pm.JobResult) {
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

		url := controller.BuildUrl(config.Main.Gid, config.Main.Nid, "result")

		reader := bytes.NewBuffer(res)
		resp, err := controller.Client.Post(url, "application/json", reader)
		if err != nil {
			log.Println("Failed to send job result to AC", url, err)
			return
		}
		resp.Body.Close()
	})

	//register the execute commands
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

	agent.RegisterHubbleFunctions(controllers, config)
	//start process mgr.
	mgr.Run()
	//System is ready to receive commands.
	//before start polling on commands, lets run our startup commands
	//from config
	for id, startup := range config.Startup {
		if startup.Args == nil {
			startup.Args = make(map[string]interface{})
		}

		cmd := &pm.Cmd{
			Gid:  config.Main.Gid,
			Nid:  config.Main.Nid,
			Id:   id,
			Name: startup.Name,
			Data: startup.Data,
			Args: pm.NewMapArgs(startup.Args),
		}

		meterInt := cmd.Args.GetInt("stats_interval")
		if meterInt == 0 {
			cmd.Args.Set("stats_interval", config.Stats.Interval)
		}

		mgr.RunCmd(cmd)
	}

	//also register extensions and run startup commands from partial configuration files
	configuration.WatchAndApply(mgr, config)

	var pollKeys []string
	if len(config.Channel.Cmds) > 0 {
		pollKeys = config.Channel.Cmds
	} else {
		pollKeys = getKeys(controllers)
	}

	pollQuery := make(url.Values)

	for _, role := range config.Main.Roles {
		pollQuery.Add("role", role)
	}

	event, _ := json.Marshal(map[string]string{
		"name": "startup",
	})

	//start pollers goroutines
	for _, key := range pollKeys {
		go func() {
			lastfail := time.Now().Unix()
			controller, ok := controllers[key]
			if !ok {
				log.Fatalf("Channel: Unknow controller '%s'", key)
			}

			client := controller.Client

			sendStartup := true

			for {
				if sendStartup {
					//this happens on first loop, or if the connection to the controller was gone and then
					//restored.
					reader := bytes.NewBuffer(event)

					url := controller.BuildUrl(config.Main.Gid, config.Main.Nid, "event")

					resp, err := client.Post(url, "application/json", reader)
					if err != nil {
						log.Println("Failed to send startup event to AC", url, err)
					} else {
						resp.Body.Close()
						sendStartup = false
					}
				}

				url := fmt.Sprintf("%s?%s", controller.BuildUrl(config.Main.Gid, config.Main.Nid, "cmd"),
					pollQuery.Encode())

				response, err := client.Get(url)
				if err != nil {
					log.Println("No new commands, retrying ...", controller.URL, err)
					//HTTP Timeout
					if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "EOF") {
						//make sure to send startup even on the next try. In case
						//agent controller was down or even booted after the agent.
						sendStartup = true
					}

					if time.Now().Unix()-lastfail < RECONNECT_SLEEP {
						time.Sleep(RECONNECT_SLEEP * time.Second)
					}
					lastfail = time.Now().Unix()

					continue
				}

				body, err := ioutil.ReadAll(response.Body)
				if err != nil {
					log.Println("Failed to load response content", err)
					continue
				}

				response.Body.Close()
				if response.StatusCode != 200 {
					log.Println("Failed to retrieve jobs", response.Status, string(body))
					time.Sleep(2 * time.Second)
					continue
				}

				if len(body) == 0 {
					//no data, can be a long poll timeout
					continue
				}

				cmd, err := pm.LoadCmd(body)
				if err != nil {
					log.Println("Failed to load cmd", err, string(body))
					continue
				}

				//set command defaults
				//1 - stats_interval
				meterInt := cmd.Args.GetInt("stats_interval")
				if meterInt == 0 {
					cmd.Args.Set("stats_interval", config.Stats.Interval)
				}

				//tag command for routing.
				ctrlConfig := config.Controllers[key]
				cmd.Args.SetTag(key)
				cmd.Args.SetController(&ctrlConfig)

				cmd.Gid = config.Main.Gid
				cmd.Nid = config.Main.Nid

				log.Println("Starting command", cmd)

				if cmd.Args.GetString("queue") == "" {
					mgr.RunCmd(cmd)
				} else {
					mgr.RunCmdQueued(cmd)
				}
			}
		}()
	}

	//wait
	select {}
}
