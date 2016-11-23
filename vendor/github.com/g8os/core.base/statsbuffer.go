package core

import (
	"github.com/g8os/core.base/stats"
	"github.com/g8os/core.base/utils"
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

type redisStatsBuffer struct {
	buffer utils.Buffer
	pool   *redis.Pool
}

func NewRedisStatsBuffer(address string, password string, capacity int, flushInt time.Duration) StatsFlusher {
	pool := utils.NewRedisPool("tcp", address, password)

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
