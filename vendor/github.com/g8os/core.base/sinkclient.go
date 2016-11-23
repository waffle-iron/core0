package core

import (
	"encoding/json"
	"fmt"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/settings"
	"github.com/g8os/core.base/utils"
	"github.com/garyburd/redigo/redis"
	"net/url"
	"strings"
)

const (
	ReturnExpire = 300
)

/*
ControllerClient represents an active agent controller connection.
*/
type sinkClient struct {
	url   string
	redis *redis.Pool
	id    string

	responseQueue string
}

/*
NewSinkClient gets a new sink connection with the given identity. Identity is used by the sink client to
introduce itself to the sink terminal.
*/
func NewSinkClient(cfg *settings.SinkConfig, id string, responseQueue ...string) (SinkClient, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "redis" {
		return nil, fmt.Errorf("expected url of format redis://<host>:<port> or redis:///unix.socket")
	}

	network := "tcp"
	address := u.Host
	if address == "" {
		network = "unix"
		address = u.Path
	}

	pool := utils.NewRedisPool(network, address, cfg.Password)

	client := &sinkClient{
		id:    id,
		url:   strings.TrimRight(cfg.URL, "/"),
		redis: pool,
	}

	if len(responseQueue) == 1 {
		client.responseQueue = responseQueue[0]
	} else if len(responseQueue) > 1 {
		return nil, fmt.Errorf("only one response queue can be provided")
	}

	return client, nil
}

func (client *sinkClient) String() string {
	return client.url
}

func (client *sinkClient) DefaultQueue() string {
	return fmt.Sprintf("core:%v",
		client.id,
	)
}

func (cl *sinkClient) GetNext(command *core.Command) error {
	db := cl.redis.Get()
	defer db.Close()

	payload, err := redis.ByteSlices(db.Do("BLPOP", cl.DefaultQueue(), 0))
	if err != nil {
		return err
	}

	return json.Unmarshal(payload[1], command)
}

func (cl *sinkClient) Respond(result *core.JobResult) error {
	if result.ID == "" {
		return fmt.Errorf("result with no ID, not pushing results back...")
	}

	db := cl.redis.Get()
	defer db.Close()

	var queue string
	if cl.responseQueue != "" {
		queue = cl.responseQueue
	} else {
		queue = fmt.Sprintf("result:%s", result.ID)
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}

	if _, err := db.Do("RPUSH", queue, payload); err != nil {
		return err
	}
	if _, err := db.Do("EXPIRE", queue, ReturnExpire); err != nil {
		return err
	}

	return nil
}
