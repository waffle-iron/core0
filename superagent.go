package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	hubble "github.com/Jumpscale/hubble/agent"
	"github.com/Jumpscale/jsagent/agent"
	"github.com/Jumpscale/jsagent/agent/lib/builtin"
	"github.com/Jumpscale/jsagent/agent/lib/logger"
	"github.com/Jumpscale/jsagent/agent/lib/pm"
	"github.com/Jumpscale/jsagent/agent/lib/stats"
	"github.com/Jumpscale/jsagent/agent/lib/utils"
	"github.com/shirou/gopsutil/process"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	RECONNECT_SLEEP  = 4
	CMD_GET_MSGS     = "get_msgs"
	CMD_OPEN_TUNNEL  = "hubble_open_tunnel"
	CMD_CLOSE_TUNNEL = "hubble_close_tunnel"
	CMD_LIST_TUNNELS = "hubble_list_tunnels"
)

type Controller struct {
	URL    string
	Client *http.Client
}

func getHttpClient(security *agent.Security) *http.Client {
	var tlsConfig tls.Config

	if security.CertificateAuthority != "" {
		pem, err := ioutil.ReadFile(security.CertificateAuthority)
		if err != nil {
			log.Fatal(err)
		}

		tlsConfig.RootCAs = x509.NewCertPool()
		tlsConfig.RootCAs.AppendCertsFromPEM(pem)
	}

	if security.ClientCertificate != "" {
		if security.ClientCertificateKey == "" {
			log.Fatal("Missing certificate key file")
		}
		// pem, err := ioutil.ReadFile(security.ClientCertificate)
		// if err != nil {
		//     log.Fatal(err)
		// }

		cert, err := tls.LoadX509KeyPair(security.ClientCertificate,
			security.ClientCertificateKey)
		if err != nil {
			log.Fatal(err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives:   true,
			Proxy:               http.ProxyFromEnvironment,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     &tlsConfig,
		},
	}
}

/*
This function will register a handler to the get_msgs function
This one is done here and NOT in the 'buildin' library because

1- we need to know where to find the db files, this will not be availble until
   the time we are registering the DB logger. If the db logger is not configured
   in the first place, then the get_msgs will not be possible.
2- Moving this register function to the build-in will cause cyclic dependencies.
*/

func registerGetMsgsFunction(path string) {
	querier := logger.NewDBMsgQuery(path)

	get_msgs := func(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
		result := pm.NewBasicJobResult(cmd)
		result.StartTime = int64(time.Duration(time.Now().UnixNano()) / time.Millisecond)

		defer func() {
			endtime := time.Duration(time.Now().UnixNano()) / time.Millisecond
			result.Time = int64(endtime) - result.StartTime
		}()

		query := logger.Query{}

		err := json.Unmarshal([]byte(cmd.Data), &query)
		if err != nil {
			log.Println("Failed to parse get_msgs query", err)
		}

		//we still can continue the query even if we have unmarshal errors.

		result_chn, err := querier.Query(query)

		if err != nil {
			result.State = pm.S_ERROR
			result.Data = fmt.Sprintf("%v", err)

			return result
		}

		records := make([]logger.Result, 0, 1000)
		for record := range result_chn {
			records = append(records, record)
		}

		data, err := json.Marshal(records)
		if err != nil {
			result.State = pm.S_ERROR
			result.Data = fmt.Sprintf("%v", err)

			return result
		}

		result.State = pm.S_SUCCESS
		result.Data = string(data)

		return result
	}

	pm.CMD_MAP[CMD_GET_MSGS] = builtin.InternalProcessFactory(get_msgs)
}

func registerHubbleFunctions(controllers map[string]Controller, settings *agent.Settings) {
	var proxisKeys []string
	if len(settings.Hubble.Controllers) == 0 {
		proxisKeys = getKeys(controllers)
	} else {
		proxisKeys = settings.Hubble.Controllers
	}

	agents := make(map[string]hubble.Agent)

	localName := fmt.Sprintf("%d.%d", settings.Main.Gid, settings.Main.Nid)
	//first of all... start all agents for controllers that are configured.
	for _, proxyKey := range proxisKeys {
		controller, ok := controllers[proxyKey]
		if !ok {
			log.Fatalf("Unknown controller '%s'", proxyKey)
		}

		//start agent for that controller.
		baseUrl := buildUrl(settings.Main.Gid, settings.Main.Nid, controller.URL, "hubble")
		parsedUrl, err := url.Parse(baseUrl)
		if err != nil {
			log.Fatal(err)
		}

		if parsedUrl.Scheme == "http" {
			parsedUrl.Scheme = "ws"
		} else if parsedUrl.Scheme == "https" {
			parsedUrl.Scheme = "wss"
		} else {
			log.Fatalf("Unknown scheme '%s'", parsedUrl.Scheme)
		}

		agent := hubble.NewAgent(parsedUrl.String(), localName, "", controller.Client.Transport.(*http.Transport).TLSClientConfig)

		agents[proxyKey] = agent

		var onExit func(agent hubble.Agent, err error)
		onExit = func(agent hubble.Agent, err error) {
			if err != nil {
				go func() {
					time.Sleep(RECONNECT_SLEEP * time.Second)
					agent.Start(onExit)
				}()
			}
		}

		agent.Start(onExit)
	}

	type TunnelData struct {
		Local   uint16 `json:"local"`
		Gateway string `json:"gateway"`
		IP      net.IP `json:"ip"`
		Remote  uint16 `json:"remote"`
	}

	openTunnle := func(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
		result := pm.NewBasicJobResult(cmd)
		result.State = pm.S_ERROR

		var tunnelData TunnelData
		err := json.Unmarshal([]byte(cmd.Data), &tunnelData)
		if err != nil {
			result.Data = fmt.Sprintf("%v", err)

			return result
		}

		if tunnelData.Gateway == localName {
			result.Data = "Can't open a tunnel to self"
			return result
		}

		tag := cmd.Args.GetTag()
		agent, ok := agents[tag]

		if !ok {
			result.Data = "Controller is not allowed to request for tunnels"
			return result
		}

		tunnel := hubble.NewTunnel(tunnelData.Local, tunnelData.Gateway, "", tunnelData.IP, tunnelData.Remote)
		err = agent.AddTunnel(tunnel)

		if err != nil {
			result.Data = fmt.Sprintf("%v", err)
			return result
		}

		tunnelData.Local = tunnel.Local()
		data, _ := json.Marshal(tunnelData)

		result.Data = string(data)
		result.Level = 20
		result.State = pm.S_SUCCESS

		return result
	}

	closeTunnel := func(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
		result := pm.NewBasicJobResult(cmd)
		result.State = pm.S_ERROR

		var tunnelData TunnelData
		err := json.Unmarshal([]byte(cmd.Data), &tunnelData)
		if err != nil {
			result.Data = fmt.Sprintf("%v", err)

			return result
		}

		tag := cmd.Args.GetTag()
		agent, ok := agents[tag]

		if !ok {
			result.Data = "Controller is not allowed to request for tunnels"
			return result
		}

		tunnel := hubble.NewTunnel(tunnelData.Local, tunnelData.Gateway, "", tunnelData.IP, tunnelData.Remote)
		agent.RemoveTunnel(tunnel)

		result.State = pm.S_SUCCESS

		return result
	}

	listTunnels := func(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
		result := pm.NewBasicJobResult(cmd)
		result.State = pm.S_ERROR

		tag := cmd.Args.GetTag()
		agent, ok := agents[tag]

		if !ok {
			result.Data = "Controller is not allowed to request for tunnels"
			return result
		}

		tunnels := agent.Tunnels()
		tunnelsInfos := make([]TunnelData, 0, len(tunnels))
		for _, t := range tunnels {
			tunnelsInfos = append(tunnelsInfos, TunnelData{
				Local:   t.Local(),
				IP:      t.IP(),
				Gateway: t.Gateway(),
				Remote:  t.Remote(),
			})
		}

		data, _ := json.Marshal(tunnelsInfos)
		result.Data = string(data)
		result.Level = 20
		result.State = pm.S_SUCCESS

		return result
	}

	pm.CMD_MAP[CMD_OPEN_TUNNEL] = builtin.InternalProcessFactory(openTunnle)
	pm.CMD_MAP[CMD_CLOSE_TUNNEL] = builtin.InternalProcessFactory(closeTunnel)
	pm.CMD_MAP[CMD_LIST_TUNNELS] = builtin.InternalProcessFactory(listTunnels)
}

func getKeys(m map[string]Controller) []string {
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

func configureLogging(mgr *pm.PM, controllers map[string]Controller, settings *agent.Settings) {
	//apply logging handlers.
	dbLoggerConfigured := false
	for _, logcfg := range settings.Logging {
		switch strings.ToLower(logcfg.Type) {
		case "db":
			if dbLoggerConfigured {
				log.Fatal("Only one db logger can be configured")
			}
			sqlFactory := logger.NewSqliteFactory(logcfg.LogDir)
			handler := logger.NewDBLogger(sqlFactory, logcfg.Levels)
			mgr.AddMessageHandler(handler.Log)
			registerGetMsgsFunction(logcfg.LogDir)

			dbLoggerConfigured = true
		case "ac":
			endpoints := make(map[string]*http.Client)

			if len(logcfg.Controllers) > 0 {
				//specific ones.
				for _, key := range logcfg.Controllers {
					controller, ok := controllers[key]
					if !ok {
						log.Fatalf("Unknow controller '%s'", key)
					}
					url := buildUrl(settings.Main.Gid, settings.Main.Nid, controller.URL, "log")
					endpoints[url] = controller.Client
				}
			} else {
				//all ACs
				for _, controller := range controllers {
					url := buildUrl(settings.Main.Gid, settings.Main.Nid, controller.URL, "log")
					endpoints[url] = controller.Client
				}
			}

			batchsize := 1000 // default
			flushint := 120   // default (in seconds)
			if logcfg.BatchSize != 0 {
				batchsize = logcfg.BatchSize
			}
			if logcfg.FlushInt != 0 {
				flushint = logcfg.FlushInt
			}

			handler := logger.NewACLogger(
				endpoints,
				batchsize,
				time.Duration(flushint)*time.Second,
				logcfg.Levels)
			mgr.AddMessageHandler(handler.Log)
		case "console":
			handler := logger.NewConsoleLogger(logcfg.Levels)
			mgr.AddMessageHandler(handler.Log)
		default:
			log.Fatalf("Unsupported logger type: %s", logcfg.Type)
		}
	}
}

func main() {
	settings := agent.Settings{}
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

	utils.LoadTomlFile(cfg, &settings)

	//loading command history file
	//history file is used to remember long running jobs during reboots.
	var history []*pm.Cmd
	hisstr, err := ioutil.ReadFile(settings.Main.HistoryFile)

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

		ioutil.WriteFile(settings.Main.HistoryFile, data, 0644)
	}

	//build list with ACs that we will poll from.
	controllers := make(map[string]Controller)
	for key, controllerCfg := range settings.Controllers {
		controllers[key] = Controller{
			URL:    controllerCfg.URL,
			Client: getHttpClient(&controllerCfg.Security),
		}
	}

	mgr := pm.NewPM(settings.Main.MessageIdFile, settings.Main.MaxJobs)

	configureLogging(mgr, controllers, &settings)

	//This handler is called every 30 sec. It should collect and report all
	//metered values needed for an external process.
	mgr.AddStatsdMeterHandler(func(statsd *stats.Statsd, cmd *pm.Cmd, ps *process.Process) {
		//for each long running external process this will be called every 2 sec
		//You can here collect all the data you want abou the process and feed
		//statsd.

		cpu, err := ps.CPUPercent(0)
		if err == nil {
			statsd.Gauage("cpu", fmt.Sprintf("%f", cpu))
		}

		mem, err := ps.MemoryInfo()
		if err == nil {
			statsd.Gauage("rss", fmt.Sprintf("%d", mem.RSS))
			statsd.Gauage("vms", fmt.Sprintf("%d", mem.VMS))
			statsd.Gauage("swap", fmt.Sprintf("%d", mem.Swap))
		}
	})

	var statsKeys []string
	if len(settings.Stats.Controllers) > 0 {
		statsKeys = settings.Stats.Controllers
	} else {
		statsKeys = getKeys(controllers)
	}

	//register handler for stats flush. Simplest impl is to send the values
	//immediately to the all ACs.
	mgr.AddStatsFlushHandler(func(stats *stats.Stats) {
		//This will be called per process per stats_interval seconds. with
		//all the aggregated stats for that process.
		res, _ := json.Marshal(stats)
		for _, key := range statsKeys {
			controller, ok := controllers[key]
			if !ok {
				log.Printf("Stats: Unknow controller '%s'\n", key)
				continue
			}

			url := buildUrl(settings.Main.Gid, settings.Main.Nid, controller.URL, "stats")
			reader := bytes.NewBuffer(res)
			resp, err := controller.Client.Post(url, "application/json", reader)
			if err != nil {
				log.Println("Failed to send stats result to AC", url, err)
				return
			}
			resp.Body.Close()
		}
	})

	var pollKeys []string
	if len(settings.Channel.Cmds) > 0 {
		pollKeys = settings.Channel.Cmds
	} else {
		pollKeys = getKeys(controllers)
	}

	pollQuery := make(url.Values)

	for _, role := range settings.Main.Roles {
		pollQuery.Add("role", role)
	}

	//start pollers goroutines
	for _, key := range pollKeys {
		go func() {
			lastfail := time.Now().Unix()
			controller, ok := controllers[key]
			if !ok {
				log.Fatalf("Channel: Unknow controller '%s'", key)
			}

			client := controller.Client

			for {
				url := fmt.Sprintf("%s?%s", buildUrl(settings.Main.Gid, settings.Main.Nid, controller.URL, "cmd"),
					pollQuery.Encode())

				response, err := client.Get(url)
				if err != nil {
					log.Println("No new commands, retrying ...", controller.URL, err)
					//HTTP Timeout
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
					cmd.Args.Set("stats_interval", settings.Stats.Interval)
				}

				//tag command for routing.
				cmd.Args.SetTag(key)
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

	//handle process results
	mgr.AddResultHandler(func(result *pm.JobResult) {
		//send result to AC.
		//NOTE: we always force the real gid and nid on the result.
		result.Gid = settings.Main.Gid
		result.Nid = settings.Main.Nid

		res, _ := json.Marshal(result)
		controller := controllers[result.Args.GetTag()]

		url := buildUrl(settings.Main.Gid, settings.Main.Nid, controller.URL, "result")

		reader := bytes.NewBuffer(res)
		resp, err := controller.Client.Post(url, "application/json", reader)
		if err != nil {
			log.Println("Failed to send job result to AC", url, err)
			return
		}
		resp.Body.Close()
	})

	//register the execute commands
	for cmdKey, cmdCfg := range settings.Cmds {
		var env []string
		if len(cmdCfg.Env) > 0 {
			env = make([]string, 0, len(cmdCfg.Env))
			for ek, ev := range cmdCfg.Env {
				env = append(env, fmt.Sprintf("%v=%v", ek, ev))
			}
		}

		pm.RegisterCmd(cmdKey, cmdCfg.Binary, cmdCfg.Cwd, cmdCfg.Script, env)
	}

	registerHubbleFunctions(controllers, &settings)
	//start process mgr.
	mgr.Run()
	//System is ready to receive commands.

	//rerun history (rerun persisted processes)
	for i := 0; i < len(history); i++ {
		cmd := history[i]
		meterInt := cmd.Args.GetInt("stats_interval")
		if meterInt == 0 {
			cmd.Args.Set("stats_interval", settings.Stats.Interval)
		}

		if err != nil {
			log.Println("Failed to load history command", history[i])
		}

		mgr.RunCmd(cmd)
	}

	// send startup event to all agent controllers
	event, _ := json.Marshal(map[string]string{
		"name": "startup",
	})

	for _, controller := range controllers {
		reader := bytes.NewBuffer(event)

		url := buildUrl(settings.Main.Gid, settings.Main.Nid, controller.URL, "event")

		resp, err := controller.Client.Post(url, "application/json", reader)
		if err != nil {
			log.Println("Failed to send startup event to AC", url, err)
			continue
		}
		resp.Body.Close()
	}

	//wait
	select {}
}
