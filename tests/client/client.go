package client

import (
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/pborman/uuid"
)

//Timeout error type
type Timeout string

func (t Timeout) Error() string {
	return "Timedout"
}

const (
	//ArgDomain domain
	ArgDomain = "domain"
	//ArgName name
	ArgName = "name"
	//ArgMaxTime max time
	ArgMaxTime = "max_time"
	//ArgMaxRestart max restart
	ArgMaxRestart = "max_restart"
	//ArgRecurringPeriod recurring period
	ArgRecurringPeriod = "recurring_period"
	//ArgStatsInterval stats interval
	ArgStatsInterval = "stats_interval"
	//ArgCmdArguments cmd arguments
	ArgCmdArguments = "args"
	//ArgQueue queue
	ArgQueue = "queue"

	//StateRunning running state
	StateRunning = "RUNNING"
	//StateQueued queued state
	StateQueued = "QUEUED"

	cmdQueueMain          = "cmds.queue"
	cmdQueueCmdQueued     = "cmd.%s.queued"
	cmdQueueAgentResponse = "cmd.%s.%d.%d"
	hashCmdResults        = "jobresult:%s"
)

//TIMEOUT timeout error
var TIMEOUT Timeout

//Job represents a job
type Job struct {
	ID        string   `json:"id"`
	Gid       int      `json:"gid"`
	Nid       int      `json:"nid"`
	Cmd       string   `json:"cmd"`
	Data      string   `json:"data"`
	Streams   []string `json:"streams"`
	Level     int      `json:"level"`
	StartTime int      `json:"starttime"`
	State     string   `json:"state"`
	Time      int      `json:"time"`
	Tags      string   `json:"tags"`

	redis *redis.Pool
}

//Command represents a command
type Command struct {
	ID     string   `json:"id"`
	Gid    int      `json:"gid"`
	Nid    int      `json:"nid"`
	Cmd    string   `json:"cmd"`
	Args   RunArgs  `json:"args"`
	Data   string   `json:"data"`
	Roles  []string `json:"roles"`
	Fanout bool     `json:"fanout"`
}

//CommandReference is an executed command
type CommandReference struct {
	ID       string
	client   Client
	iterator int
}

//RunArgs holds the execution arguments
type RunArgs map[string]interface{}

//Client interface
type Client interface {
	Run(cmd *Command) (*CommandReference, error)
	GetJobs(ID string, timeout int) ([]*Job, error)
}

//NewRunArgs creates a new run arguments
func NewRunArgs(domain string, name string, maxTime int, maxRestart int,
	recurrintPeriod int, statsInterval int, args []string, queue string) RunArgs {
	runArgs := make(RunArgs)
	runArgs[ArgDomain] = domain
	runArgs[ArgName] = name
	runArgs[ArgMaxTime] = maxTime
	runArgs[ArgMaxRestart] = maxRestart
	runArgs[ArgRecurringPeriod] = recurrintPeriod
	runArgs[ArgStatsInterval] = statsInterval
	runArgs[ArgCmdArguments] = args
	// runArgs["loglevels"] = loglevels
	// runArgs["loglevels_db"] = loglevelsDB
	// runArgs["loglevels_ac"] = loglevelsAC
	runArgs[ArgQueue] = queue

	return runArgs
}

//NewDefaultRunArgs creates a new default run arguments with default values
func NewDefaultRunArgs() RunArgs {
	return NewRunArgs("", "", 0, 0, 0, 0, []string{}, "")
}

//Domain domain
func (args RunArgs) Domain() string {
	return args[ArgDomain].(string)
}

//Name name
func (args RunArgs) Name() string {
	return args[ArgName].(string)
}

//MaxTime max time to run
func (args RunArgs) MaxTime() int {
	return args[ArgMaxTime].(int)
}

//MaxRestart max number of restart before giving up
func (args RunArgs) MaxRestart() int {
	return args[ArgMaxRestart].(int)
}

//RecurringPeriod recurring period
func (args RunArgs) RecurringPeriod() int {
	return args[ArgRecurringPeriod].(int)
}

//StatsInterval stats interval
func (args RunArgs) StatsInterval() int {
	return args[ArgStatsInterval].(int)
}

//Args command line arguments (if needed)
func (args RunArgs) Args() []string {
	return args[ArgCmdArguments].([]string)
}

//Queue queue name for serial execution
func (args RunArgs) Queue() string {
	return args[ArgQueue].(string)
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

//New creates a new client
func New(addr string, password string) Client {
	return &clientImpl{
		redis: newPool(addr, password),
	}
}

//Run runs a command on client
func (client *clientImpl) Run(cmd *Command) (*CommandReference, error) {
	cmd.ID = uuid.New()
	ref := &CommandReference{
		ID:     cmd.ID,
		client: client,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}

	db := client.redis.Get()
	defer db.Close()

	_, err = db.Do("RPUSH", cmdQueueMain, data)
	if err != nil {
		return nil, err
	}

	return ref, nil
}

//Wait waits for job until response is ready
func (job *Job) Wait(timeout int) error {
	db := job.redis.Get()
	defer db.Close()

	//only wait if state in running or queued state.
	if job.State != StateRunning && job.State != StateQueued {
		return nil
	}

	queue := fmt.Sprintf(cmdQueueAgentResponse, job.ID, job.Gid, job.Nid)
	data, err := db.Do("BRPOPLPUSH", queue, queue, timeout)
	if err != nil {
		return err
	}

	if data == nil {
		return TIMEOUT
	}
	payload, err := redis.String(data, err)

	err = json.Unmarshal([]byte(payload), job)

	if err != nil {
		return err
	}

	return nil
}

//GetJobs returns all the command jobs
func (client *clientImpl) GetJobs(ID string, timeout int) ([]*Job, error) {
	db := client.redis.Get()
	defer db.Close()

	queue := fmt.Sprintf(cmdQueueCmdQueued, ID)
	data, err := db.Do("BRPOPLPUSH", queue, queue, timeout)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, TIMEOUT
	}
	resultQueue := fmt.Sprintf(hashCmdResults, ID)
	jobsdata, err := redis.StringMap(db.Do("HGETALL", resultQueue))

	if err != nil {
		return nil, err
	}

	results := make([]*Job, 0, 10)
	for _, jobdata := range jobsdata {
		result := &Job{}
		if err := json.Unmarshal([]byte(jobdata), result); err != nil {
			return nil, err
		}

		result.redis = client.redis
		results = append(results, result)
	}

	return results, nil
}

//GetNextResult returns the next available result
func (ref *CommandReference) GetNextResult(timeout int) (*Job, error) {
	jobs, err := ref.client.GetJobs(ref.ID, timeout)

	if err != nil {
		return nil, err
	}

	if ref.iterator >= len(jobs) {
		return nil, fmt.Errorf("No more jobs")
	}

	job := jobs[ref.iterator]
	ref.iterator++

	return job, job.Wait(timeout)
}

//GetJobs get command jobs
func (ref *CommandReference) GetJobs(timeout int) ([]*Job, error) {
	return ref.client.GetJobs(ref.ID, timeout)
}
