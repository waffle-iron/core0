package agent

import (
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/builtin"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/settings"
	hubble "github.com/Jumpscale/hubble/agent"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	RECONNECT_SLEEP = 4

	CMD_OPEN_TUNNEL  = "hubble_open_tunnel"
	CMD_CLOSE_TUNNEL = "hubble_close_tunnel"
	CMD_LIST_TUNNELS = "hubble_list_tunnels"
)

func RegisterHubbleFunctions(controllers map[string]*ControllerClient, settings *settings.Settings) {
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
		baseUrl := controller.BuildUrl(settings.Main.Gid, settings.Main.Nid, "hubble")
		parsedUrl, err := url.Parse(baseUrl)
		if err != nil {
			log.Fatal(err)
		}

		if parsedUrl.Scheme == "http" {
			parsedUrl.Scheme = "ws"
		} else if parsedUrl.Scheme == "https" {
			parsedUrl.Scheme = "wss"
		} else {
			log.Fatalf("Unknown scheme '%s' in controller url '%s'", parsedUrl.Scheme, controller.URL)
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
		Tag     string `json:"controller,omitempty"`
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
		if tag == "" {
			//this can only happing if the open tunnel command is coming from
			//a startup config. So only support setting up tag from config and
			//can't be set from normal commands for security.
			tag = tunnelData.Tag
		}

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
		result.Level = pm.L_RESULT_JSON
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
		if tag == "" {
			//this can only happing if the open tunnel command is coming from
			//a startup config. So only support setting up tag from config and
			//can't be set from normal commands for security.
			tag = tunnelData.Tag
		}
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
		result.Level = pm.L_RESULT_JSON
		result.State = pm.S_SUCCESS

		return result
	}

	pm.CMD_MAP[CMD_OPEN_TUNNEL] = builtin.InternalProcessFactory(openTunnle)
	pm.CMD_MAP[CMD_CLOSE_TUNNEL] = builtin.InternalProcessFactory(closeTunnel)
	pm.CMD_MAP[CMD_LIST_TUNNELS] = builtin.InternalProcessFactory(listTunnels)
}
