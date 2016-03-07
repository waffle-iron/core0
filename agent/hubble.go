package agent

import (
	"encoding/json"
	"fmt"
	hubble "github.com/Jumpscale/hubble/agent"
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/pm/process"
	"github.com/g8os/core/agent/lib/settings"
	"net/http"
	"net/url"
	"time"
)

const (
	//ReconnectSleepTime Sleeps this amount of seconds if hubble agent connection failed before retyring
	ReconnectSleepTime = 4

	cmdOpenTunnel  = "hubble_open_tunnel"
	cmdCLoseTunnel = "hubble_close_tunnel"
	cmdListTunnels = "hubble_list_tunnels"
)

type hubbleFunc struct {
	name   string
	agents map[string]hubble.Agent
}

type tunnelData struct {
	Local   uint16 `json:"local"`
	Gateway string `json:"gateway"`
	IP      string `json:"ip"`
	Remote  uint16 `json:"remote"`
	Tag     string `json:"controller,omitempty"`
}

func (fnc *hubbleFunc) openTunnle(cmd *core.Cmd) (interface{}, error) {
	var tunnelData tunnelData
	err := json.Unmarshal([]byte(cmd.Data), &tunnelData)
	if err != nil {
		return nil, err
	}

	if tunnelData.Gateway == fnc.name {
		return nil, fmt.Errorf("Can't open a tunnel to self")
	}

	tag := cmd.Args.GetTag()
	if tag == "" {
		//this can only happing if the open tunnel command is coming from
		//a startup config. So only support setting up tag from config and
		//can't be set from normal commands for security.
		tag = tunnelData.Tag
	}

	agent, ok := fnc.agents[tag]

	if !ok {
		return nil, fmt.Errorf("Controller is not allowed to request for tunnels")
	}

	tunnel := hubble.NewTunnel(tunnelData.Local, tunnelData.Gateway, "", tunnelData.IP, tunnelData.Remote)
	err = agent.AddTunnel(tunnel)

	if err != nil {
		return nil, err
	}

	tunnelData.Local = tunnel.Local()
	return tunnelData, nil
}

func (fnc *hubbleFunc) closeTunnel(cmd *core.Cmd) (interface{}, error) {
	var tunnelData tunnelData
	err := json.Unmarshal([]byte(cmd.Data), &tunnelData)
	if err != nil {
		return nil, err
	}

	tag := cmd.Args.GetTag()
	if tag == "" {
		//this can only happing if the open tunnel command is coming from
		//a startup config. So only support setting up tag from config and
		//can't be set from normal commands for security.
		tag = tunnelData.Tag
	}
	agent, ok := fnc.agents[tag]

	if !ok {
		return nil, fmt.Errorf("Controller is not allowed to request for tunnels")
	}

	tunnel := hubble.NewTunnel(tunnelData.Local, tunnelData.Gateway, "", tunnelData.IP, tunnelData.Remote)
	agent.RemoveTunnel(tunnel)

	return true, nil
}

func (fnc *hubbleFunc) listTunnels(cmd *core.Cmd) (interface{}, error) {
	tag := cmd.Args.GetTag()
	agent, ok := fnc.agents[tag]

	if !ok {
		return nil, fmt.Errorf("Controller is not allowed to request for tunnels")
	}

	tunnels := agent.Tunnels()
	tunnelsInfos := make([]tunnelData, 0, len(tunnels))
	for _, t := range tunnels {
		tunnelsInfos = append(tunnelsInfos, tunnelData{
			Local:   t.Local(),
			IP:      t.Host(),
			Gateway: t.Gateway(),
			Remote:  t.RemotePort(),
		})
	}

	return tunnelsInfos, nil
}

/*
RegisterHubbleFunctions Registers all the handlers for hubble commands this include
- hubble_open_tunnel
- hubble_close_tunnel
- hubble_list_tunnels

*/
func RegisterHubbleFunctions(controllers map[string]*settings.ControllerClient) {
	var proxisKeys []string
	if len(settings.Settings.Hubble.Controllers) == 0 {
		proxisKeys = getKeys(controllers)
	} else {
		proxisKeys = settings.Settings.Hubble.Controllers
	}

	agents := make(map[string]hubble.Agent)

	localName := fmt.Sprintf("%d.%d", settings.Options.Gid(), settings.Options.Nid())
	//first of all... start all agents for controllers that are configured.
	for _, proxyKey := range proxisKeys {
		controller, ok := controllers[proxyKey]
		if !ok {
			log.Fatalf("Unknown controller '%s'", proxyKey)
		}

		//start agent for that controller.
		baseURL := controller.BuildURL("hubble")
		parsedURL, err := url.Parse(baseURL)
		if err != nil {
			log.Fatal(err)
		}

		if parsedURL.Scheme == "http" {
			parsedURL.Scheme = "ws"
		} else if parsedURL.Scheme == "https" {
			parsedURL.Scheme = "wss"
		} else {
			log.Fatalf("Unknown scheme '%s' in controller url '%s'", parsedURL.Scheme, controller.URL)
		}

		agent := hubble.NewAgent(parsedURL.String(), localName, "", controller.Client.Transport.(*http.Transport).TLSClientConfig)

		agents[proxyKey] = agent

		var onExit func(agent hubble.Agent, err error)
		onExit = func(agent hubble.Agent, err error) {
			if err != nil {
				go func() {
					time.Sleep(ReconnectSleepTime * time.Second)
					agent.Start(onExit)
				}()
			}
		}

		agent.Start(onExit)
	}

	fncs := &hubbleFunc{
		name:   localName,
		agents: agents,
	}

	pm.CmdMap[cmdOpenTunnel] = process.NewInternalProcessFactory(fncs.openTunnle)
	pm.CmdMap[cmdCLoseTunnel] = process.NewInternalProcessFactory(fncs.closeTunnel)
	pm.CmdMap[cmdListTunnels] = process.NewInternalProcessFactory(fncs.listTunnels)
}
