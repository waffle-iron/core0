package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/garyburd/redigo/redis"
)

const (
	ARG_DOMAIN           = "domain"
	ARG_NAME             = "name"
	ARG_MAX_TIME         = "max_time"
	ARG_MAX_RESTART      = "max_restart"
	ARG_RECURRING_PERIOD = "recurring_period"
	ARG_STATS_INTERVAL   = "stats_interval"
	ARG_CMD_ARGS         = "args"
	ARG_QUEUE            = "queue"
)

type Result struct {
	Id        string `json:"id"`
	Gid       int    `json:"gid"`
	Nid       int    `json:"nid"`
	Cmd       string `json:"cmd"`
	Data      string `json:"data"`
	Level     int    `json:"level"`
	Starttime int    `json:"starttime"`
	State     string `json:"state"`
	Time      int    `json:"time"`
}

type Command struct {
	Id   string  `json:"id"`
	Gid  int     `json:"gid"`
	Nid  int     `json:"nid"`
	Cmd  string  `json:"cmd"`
	Args RunArgs `json:"args"`
	Data string  `json:"data"`
	Role string  `json:"role"`
}

type CommandReference struct {
	Id     string
	client Client
}

type RunArgs map[string]interface{}

type Client interface {
	Run(cmd *Command) (*CommandReference, error)
	Result(ref *CommandReference, timeout int) (*Result, error)
}

func NewRunArgs(domain string, name string, maxTime int, maxRestart int,
	recurrintPeriod int, statsInterval int, args []string, queue string) RunArgs {
	runArgs := make(RunArgs)
	runArgs[ARG_DOMAIN] = domain
	runArgs[ARG_NAME] = name
	runArgs[ARG_MAX_TIME] = maxTime
	runArgs[ARG_MAX_RESTART] = maxRestart
	runArgs[ARG_RECURRING_PERIOD] = recurrintPeriod
	runArgs[ARG_STATS_INTERVAL] = statsInterval
	runArgs[ARG_CMD_ARGS] = args
	// runArgs["loglevels"] = loglevels
	// runArgs["loglevels_db"] = loglevelsDB
	// runArgs["loglevels_ac"] = loglevelsAC
	runArgs[ARG_QUEUE] = queue

	return runArgs
}

func NewDefaultRunArgs() RunArgs {
	return NewRunArgs("", "", 0, 0, 0, 0, []string{}, "")
}

func (args RunArgs) Domain() string {
	return args[ARG_DOMAIN].(string)
}

func (args RunArgs) Name() string {
	return args[ARG_NAME].(string)
}

func (args RunArgs) MaxTime() int {
	return args[ARG_MAX_TIME].(int)
}

func (args RunArgs) MaxRestart() int {
	return args[ARG_MAX_RESTART].(int)
}

func (args RunArgs) RecurringPeriod() int {
	return args[ARG_RECURRING_PERIOD].(int)
}

func (args RunArgs) StatsInterval() int {
	return args[ARG_STATS_INTERVAL].(int)
}

func (args RunArgs) Args() []string {
	return args[ARG_CMD_ARGS].([]string)
}

func (args RunArgs) Queue() string {
	return args[ARG_QUEUE].(string)
}

func newPool(addr string, password string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:   80,
		MaxActive: 12000,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", addr)

			if err != nil {
				panic(err.Error())
			}

			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}

			return c, err
		},
	}
}

type clientImpl struct {
	redis *redis.Pool
}

func New(addr string, password string) Client {
	return &clientImpl{
		redis: newPool(addr, password),
	}
}

func (client *clientImpl) Run(cmd *Command) (*CommandReference, error) {
	ref := &CommandReference{
		Id:     cmd.Id,
		client: client,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}

	db := client.redis.Get()
	defer db.Close()

	_, err = db.Do("RPUSH", "cmds_queue", data)
	if err != nil {
		return nil, err
	}

	return ref, nil
}

func (client *clientImpl) Result(ref *CommandReference, timeout int) (*Result, error) {
	db := client.redis.Get()
	defer db.Close()

	data, err := db.Do("BLPOP", fmt.Sprintf("cmds_queue_%s", ref.Id), timeout)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, errors.New("Timedout waiting for response")
	}

	payload := data.([]interface{})[1]
	result := &Result{}

	err = json.Unmarshal(payload.([]byte), result)

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (ref *CommandReference) Result(timeout int) (*Result, error) {
	return ref.client.Result(ref, timeout)
}
