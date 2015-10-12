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
	"github.com/Jumpscale/agent2/agent/lib/stats"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"github.com/shirou/gopsutil/process"
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

func buildUrl(gid int, nid int, base string, endpoint string) string {
	base = strings.TrimRight(base, "/")
	return fmt.Sprintf("%s/%d/%d/%s", base,
		gid,
		nid,
		endpoint)
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

	//loading command history file
	//history file is used to remember long running jobs during reboots.
	var history []*pm.Cmd
	hisstr, err := ioutil.ReadFile(config.Main.HistoryFile)

	if err == nil {
		err = json.Unmarshal(hisstr, &history)
		if err != nil {
			log.Println("Failed to load history file, invalid syntax ", err)
			history = make([]*pm.Cmd, 0)
		}
	} else {
		log.Println("Couldn't read history file")
		history = make([]*pm.Cmd, 0)
	}

	//dump hisory file
	dumpHistory := func() {
		data, err := json.Marshal(history)
		if err != nil {
			log.Fatal("Failed to write history file")
		}

		ioutil.WriteFile(config.Main.HistoryFile, data, 0644)
	}

	//build list with ACs that we will poll from.
	controllers := make(map[string]*agent.ControllerClient)
	for key, controllerCfg := range config.Controllers {
		controllers[key] = agent.NewControllerClient(controllerCfg)
	}

	mgr := pm.NewPM(config.Main.MessageIdFile, config.Main.MaxJobs)

	logger.ConfigureLogging(mgr, controllers, config)

	//This handler is called every 30 sec. It should collect and report all
	//metered values needed for an external process.
	mgr.AddStatsdMeterHandler(func(statsd *stats.Statsd, cmd *pm.Cmd, ps *process.Process) {
		//for each long running external process this will be called every 2 sec
		//You can here collect all the data you want abou the process and feed
		//statsd.

		cpu, err := ps.CPUPercent(0)
		if err == nil {
			statsd.Gauage("__cpu__", fmt.Sprintf("%f", cpu))
		}

		mem, err := ps.MemoryInfo()
		if err == nil {
			statsd.Gauage("__rss__", fmt.Sprintf("%d", mem.RSS))
			statsd.Gauage("__vms__", fmt.Sprintf("%d", mem.VMS))
			statsd.Gauage("__swap__", fmt.Sprintf("%d", mem.Swap))
		}
	})

	var statsDestinations []string
	if len(config.Stats.Controllers) > 0 {
		statsDestinations = config.Stats.Controllers
	} else {
		statsDestinations = getKeys(controllers)
	}

	//build a buffer for statsd messages (which are comming from each single command)
	//and buffer them so we only send them to AC if we have a 1000 record, or reached
	//a time of 60 seconds.
	statsBuffer := utils.NewBuffer(1000, 120*time.Second, func(stats []interface{}) {
		log.Println("Flushing stats to AC", len(stats))
		if len(stats) == 0 {
			return
		}

		res, _ := json.Marshal(stats)
		for _, key := range statsDestinations {
			controller, ok := controllers[key]
			if !ok {
				log.Printf("Stats: Unknow controller '%s'\n", key)
				continue
			}

			url := buildUrl(config.Main.Gid, config.Main.Nid, controller.URL, "stats")
			reader := bytes.NewBuffer(res)
			resp, err := controller.Client.Post(url, "application/json", reader)
			if err != nil {
				log.Println("Failed to send stats result to AC", url, err)
				return
			}
			resp.Body.Close()
		}
	})
	//register handler for stats flush. Simplest impl is to send the values
	//immediately to the all ACs.
	mgr.AddStatsFlushHandler(func(stats *stats.Stats) {
		//This will be called per process per stats_interval seconds. with
		//all the aggregated stats for that process.
		statsBuffer.Append(stats)
	})

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

		url := buildUrl(config.Main.Gid, config.Main.Nid, controller.URL, "result")

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

					url := buildUrl(config.Main.Gid, config.Main.Nid, controller.URL, "event")

					resp, err := client.Post(url, "application/json", reader)
					if err != nil {
						log.Println("Failed to send startup event to AC", url, err)
					} else {
						resp.Body.Close()
						sendStartup = false
					}
				}

				url := fmt.Sprintf("%s?%s", buildUrl(config.Main.Gid, config.Main.Nid, controller.URL, "cmd"),
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

				if cmd.Args.GetInt("max_time") == -1 {
					//that's a long running process.
					history = append(history, cmd)
					dumpHistory()
				}

				if cmd.Args.GetString("queue") == "" {
					mgr.RunCmd(cmd)
				} else {
					mgr.RunCmdQueued(cmd)
				}
			}
		}()
	}

	//rerun history (rerun persisted processes)
	for i := 0; i < len(history); i++ {
		cmd := history[i]
		meterInt := cmd.Args.GetInt("stats_interval")
		if meterInt == 0 {
			cmd.Args.Set("stats_interval", config.Stats.Interval)
		}

		if err != nil {
			log.Println("Failed to load history command", history[i])
		}

		mgr.RunCmd(cmd)
	}

	// send startup event to all agent controllers

	//wait
	select {}
}
