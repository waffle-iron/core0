package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/settings"
	"github.com/Jumpscale/agent2/agent/lib/stats"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"log"
	"time"
)

type StatsBuffer struct {
	gid          int
	nid          int
	destinations []*ControllerClient
	buffer       utils.Buffer
}

func NewStatsBuffer(capacity int, flushInt time.Duration, controllers map[string]*ControllerClient,
	config *settings.Settings) *StatsBuffer {
	var destKeys []string
	if len(config.Stats.Controllers) > 0 {
		destKeys = config.Stats.Controllers
	} else {
		destKeys = getKeys(controllers)
	}

	destinations := make([]*ControllerClient, 0, len(destKeys))
	for _, key := range destKeys {
		controller, ok := controllers[key]
		if !ok {
			panic(fmt.Sprintf("Unknown controller '%s' while configurint statsd", key))
		}

		destinations = append(destinations, controller)
	}

	buffer := &StatsBuffer{
		gid:          config.Main.Gid,
		nid:          config.Main.Nid,
		destinations: destinations,
	}

	buffer.buffer = utils.NewBuffer(1000, 120*time.Second, buffer.onflush)

	return buffer
}

func (buffer *StatsBuffer) onflush(stats []interface{}) {
	log.Println("Flushing stats to AC", len(stats))
	if len(stats) == 0 {
		return
	}

	res, _ := json.Marshal(stats)
	for _, controller := range buffer.destinations {
		url := controller.BuildUrl(buffer.gid, buffer.nid, "stats")
		reader := bytes.NewBuffer(res)
		resp, err := controller.Client.Post(url, "application/json", reader)
		if err != nil {
			log.Println("Failed to send stats result to AC", url, err)
			return
		}
		resp.Body.Close()
	}
}

func (buffer *StatsBuffer) Handler(stats *stats.Stats) {
	buffer.buffer.Append(stats)
}
