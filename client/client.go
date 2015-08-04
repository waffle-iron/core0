package client

import (
	"github.com/garyburd/redigo/redis"
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

type RunArgs map[string]interface{}

type Client interface {
	Run(cmd *Command)
}

func NewRunArgs(domain string, name string, maxTime int, maxRestart int,
	recurrintPeriod int, statsInterval int, args []string, loglevels string,
	loglevelsDB string, loglevelsAC string, queue string) RunArgs {
	runArgs := make(RunArgs)
	runArgs["domain"] = domain
	runArgs["name"] = name
	runArgs["max_time"] = maxTime
	runArgs["max_restart"] = maxRestart
	runArgs["recurrint_period"] = recurrintPeriod
	runArgs["stats_interval"] = statsInterval
	runArgs["args"] = args
	runArgs["loglevels"] = loglevels
	runArgs["loglevels_db"] = loglevelsDB
	runArgs["loglevels_ac"] = loglevelsAC
	runArgs["queue"] = queue

	return runArgs
}

func (args RunArgs) Domain() string {
	return args["domain"].(string)
}

func (args RunArgs) Name() string {
	return args["name"].(string)
}

func (args RunArgs) MaxTime() int {
	return args["max_time"].(int)
}

func (args RunArgs) MaxRestart() int {
	return args["max_restart"].(int)
}

func (args RunArgs) RecurringPeriod() int {
	return args["recurrint_period"].(int)
}

func (args RunArgs) StatsInterval() int {
	return args["stats_interval"].(int)
}

func (args RunArgs) Args() []string {
	return args["args"].([]string)
}

func (args RunArgs) Loglevels() string {
	return args["loglevels"].(string)
}

func (args RunArgs) loglevelsDB() string {
	return args["loglevels_db"].(string)
}

func (args RunArgs) loglevelsAC() string {
	return args["loglevels_ac"].(string)
}

func (args RunArgs) Queue() string {
	return args["queue"].(string)
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

func (client *clientImpl) Run(cmd *Command) {

}
