package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/pm/core"
	"github.com/g8os/core/agent/lib/settings"
	"io/ioutil"
	"net/url"
	"strings"
	"time"
)

type poller struct {
	key        string
	controller *settings.ControllerClient
}

func getKeys(m map[string]*settings.ControllerClient) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	return keys
}

func newPoller(key string, controller *settings.ControllerClient) *poller {
	poll := &poller{
		key:        key,
		controller: controller,
	}

	return poll
}

func (poll *poller) longPoll() {
	lastfail := time.Now().Unix()
	controller := poll.controller
	client := controller.Client
	config := settings.Settings

	sendStartup := true

	event, _ := json.Marshal(map[string]string{
		"name": "startup",
	})

	pollQuery := make(url.Values)

	for _, role := range settings.Options.Roles() {
		pollQuery.Add("role", role)
	}

	pollURL := fmt.Sprintf("%s?%s", controller.BuildURL("cmd"),
		pollQuery.Encode())

	for {
		if sendStartup {
			//this happens on first loop, or if the connection to the controller was gone and then
			//restored.
			reader := bytes.NewBuffer(event)

			url := controller.BuildURL("event")

			resp, err := client.Post(url, "application/json", reader)
			if err != nil {
				log.Warningf("Failed to send startup event to controller '%s': %s", url, err)
			} else {
				resp.Body.Close()
				sendStartup = false
			}
		}

		response, err := client.Get(pollURL)
		if err != nil {
			log.Infof("No new commands, retrying ... '%s' [%s]", controller.URL, err)
			//HTTP Timeout
			if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "EOF") {
				//make sure to send startup even on the next try. In case
				//agent controller was down or even booted after the agent.
				sendStartup = true
			}

			if time.Now().Unix()-lastfail < ReconnectSleepTime {
				time.Sleep(ReconnectSleepTime * time.Second)
			}
			lastfail = time.Now().Unix()

			continue
		}

		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Errorf("Failed to load response content: %s", err)
			continue
		}

		response.Body.Close()
		if response.StatusCode != 200 {
			log.Errorf("Failed to retrieve jobs (%s): %s", response.Status, string(body))
			time.Sleep(2 * time.Second)
			continue
		}

		if len(body) == 0 {
			//no data, can be a long poll timeout
			continue
		}

		cmd, err := core.LoadCmd(body)
		if err != nil {
			log.Errorf("Failed to load cmd (%s): %s", err, string(body))
			continue
		}

		//set command defaults
		//1 - stats_interval
		meterInt := cmd.Args.GetInt("stats_interval")
		if meterInt == 0 {
			cmd.Args.Set("stats_interval", config.Stats.Interval)
		}

		//tag command for routing.
		ctrlConfig := controller.Config
		cmd.Args.SetTag(poll.key)
		cmd.Args.SetController(ctrlConfig)

		cmd.Gid = settings.Options.Gid()
		cmd.Nid = settings.Options.Nid()

		log.Infof("Starting command %s", cmd)

		if cmd.Args.GetString("queue") == "" {
			pm.GetManager().PushCmd(cmd)
		} else {
			pm.GetManager().PushCmdToQueue(cmd)
		}
	}
}

/*
StartPollers starts the long polling routines and feed the manager with received commands
*/
func StartPollers(controllers map[string]*settings.ControllerClient) {
	var keys []string
	if len(settings.Settings.Channel.Cmds) > 0 {
		keys = settings.Settings.Channel.Cmds
	} else {
		keys = getKeys(controllers)
	}

	for _, key := range keys {
		controller, ok := controllers[key]
		if !ok {
			log.Fatalf("No contoller with name '%s'", key)
		}

		poll := newPoller(key, controller)
		go poll.longPoll()
	}
}
