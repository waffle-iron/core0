package logger

import (
	"encoding/json"
	"github.com/Jumpscale/agent8/agent/lib/pm/core"
	"github.com/Jumpscale/agent8/agent/lib/pm/stream"
	"github.com/Jumpscale/agent8/agent/lib/utils"
	"github.com/garyburd/redigo/redis"
	"log"
	"time"
)

const (
	RedisLoggerQueue = "agent.logs"
)

type redisLogger struct {
	pool     *redis.Pool
	defaults []int
}

func NewRedisLogger(address string, password string, defaults []int) Logger {
	return &redisLogger{
		pool:     utils.NewRedisPool(address, password),
		defaults: defaults,
	}
}

func (l *redisLogger) Log(cmd *core.Cmd, msg *stream.Message) {
	if len(l.defaults) > 0 && !utils.In(l.defaults, msg.Level) {
		return
	}

	db := l.pool.Get()
	defer db.Close()

	data := make(map[string]interface{})
	data["epoch"] = msg.Epoch / int64(time.Millisecond)
	data["id"] = msg.ID
	data["message"] = msg.Message
	data["level"] = msg.Level
	data["jobid"] = cmd.ID

	bytes, err := json.Marshal(data)
	if err != nil {
		log.Println("Failed to serialize message for redis logger", err)
		return
	}

	if err := db.Send("RPUSH", RedisLoggerQueue, bytes); err != nil {
		log.Println("Failed to push log message to redis", err)
	}
}
