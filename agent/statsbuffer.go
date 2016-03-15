package agent

import (
	"bytes"
	"encoding/json"
	"github.com/g8os/core/agent/lib/settings"
	"github.com/g8os/core/agent/lib/stats"
	"github.com/g8os/core/agent/lib/utils"
	"github.com/garyburd/redigo/redis"
	"time"
)

const (
	RedisStatsQueue = "agent.stats"
)

/*
StatsBuffer implements a buffering and flushing mechanism to buffer statsd messages
that are collected via the process manager. Flush happens when buffer is full or a certain time passes since last flush.

The StatsBuffer.Handler should be registers as StatsFlushHandler on the process manager object.
*/
type StatsFlusher interface {
	Handler(stats *stats.Stats)
}

type acStatsBuffer struct {
	destinations []*settings.ControllerClient
	buffer       utils.Buffer
}

/*
NewStatsBuffer creates new StatsBuffer
*/
func NewACStatsBuffer(capacity int, flushInt time.Duration, controllers map[string]*settings.ControllerClient) StatsFlusher {
	var destKeys []string
	if len(settings.Settings.Stats.Ac.Controllers) > 0 {
		destKeys = settings.Settings.Stats.Ac.Controllers
	} else {
		destKeys = getKeys(controllers)
	}

	destinations := make([]*settings.ControllerClient, 0, len(destKeys))
	for _, key := range destKeys {
		controller, ok := controllers[key]
		if !ok {
			log.Fatalf("Unknown controller '%s' while configurint statsd", key)
		}

		destinations = append(destinations, controller)
	}

	buffer := &acStatsBuffer{
		destinations: destinations,
	}

	buffer.buffer = utils.NewBuffer(1000, 120*time.Second, buffer.onflush)

	return buffer
}

func (buffer *acStatsBuffer) onflush(stats []interface{}) {
	log.Debugf("Flushing stats to controller '%d'", len(stats))
	if len(stats) == 0 {
		return
	}

	res, _ := json.Marshal(stats)
	for _, controller := range buffer.destinations {
		url := controller.BuildURL("stats")
		reader := bytes.NewBuffer(res)
		resp, err := controller.Client.Post(url, "application/json", reader)
		if err != nil {
			log.Errorf("Failed to send stats result to controller '%s': %s", url, err)
			return
		}
		resp.Body.Close()
	}
}

/*
Handler should be used as a handler to manager stats messages. This method will buffer the feed messages
to be flused on time.
*/
func (buffer *acStatsBuffer) Handler(stats *stats.Stats) {
	buffer.buffer.Append(stats)
}

type redisStatsBuffer struct {
	buffer utils.Buffer
	pool   *redis.Pool
}

func NewRedisStatsBuffer(address string, password string, capacity int, flushInt time.Duration) StatsFlusher {
	pool := utils.NewRedisPool(address, password)

	redisBuffer := &redisStatsBuffer{
		pool: pool,
	}

	redisBuffer.buffer = utils.NewBuffer(capacity, flushInt, redisBuffer.onFlush)

	return redisBuffer
}

func (r *redisStatsBuffer) Handler(stats *stats.Stats) {
	r.buffer.Append(stats)
}

func (r *redisStatsBuffer) onFlush(stats []interface{}) {
	if len(stats) == 0 {
		return
	}

	db := r.pool.Get()
	defer db.Close()

	call := []interface{}{RedisStatsQueue}
	call = append(call, stats...)

	if err := db.Send("RPUSH", call...); err != nil {
		log.Errorf("Failed to push stats messages to redis: %s", err)
	}
}
