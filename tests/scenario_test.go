package agent

import (
	"fmt"
	"github.com/Jumpscale/jsagent/agent/lib/utils"
	"github.com/Jumpscale/jsagent/client"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"
	"testing"
)

const (
	NUM_AGENT = 2
)

var AGENT_TMP string = `
[main]
gid = {gid}
nid = 1
max_jobs = 100
message_id_file = "./.mid"
history_file = "./.history"
roles = {roles}

[controllers]
    [controllers.main]
    url = "http://localhost:1221"
`

var CONTROLLER_TMP string = `
[main]
listen = ":1221"
redis_host =  "127.0.0.1:6379"
redis_password = ""

[influxdb]
host = "127.0.0.1:8086"
db   = "agentcontroller"
user = "ac"
password = "acctrl"

[handlers]
binary = "python2.7"
cwd = "./handlers"
    [handlers.env]
    PYTHONPATH = "/opt/jumpscale7/lib:../client"
    SYNCTHING_URL = "http://localhost:8384/"
    SYNCTHING_SHARED_FOLDER_ID = "jumpscripts"
    #SYNCTHING_API_KEY = ""
    REDIS_ADDRESS = "localhost"
    REDIS_PORT = "6379"
    #REDIS_PASSWORD = ""

`

var AGENT_ROLES = map[int]string{
	1: `["cpu"]`,
	2: `["cpu", "storage"]`,
}

func TestMain(m *testing.M) {
	//start ac, and 2 agents.
	goPath := os.Getenv("GOPATH")
	agentPath := path.Join(goPath, "bin", "jsagent")
	controllerPath := path.Join(goPath, "bin", "jsagentcontroller")

	{
		//build controller
		log.Println("Building agent controller")
		cmd := exec.Command("go", "install", "github.com/Jumpscale/jsagentcontroller")
		if err := cmd.Run(); err != nil {
			log.Fatal("Failed to buld controller", err)
		}
	}

	{
		//build agent
		log.Println("Building agent")
		cmd := exec.Command("go", "install", "github.com/Jumpscale/jsagent")
		if err := cmd.Run(); err != nil {
			log.Fatal("Failed to build agent", err)
		}
	}

	//start controller.
	var wg sync.WaitGroup
	wg.Add(NUM_AGENT + 1)

	ctrlCfg := utils.Format(CONTROLLER_TMP, map[string]interface{}{})

	ctrlCfgPath := "/tmp/ac.toml"

	ioutil.WriteFile(ctrlCfgPath, []byte(ctrlCfg), 0644)

	controller := exec.Command(controllerPath, "-c", ctrlCfgPath)
	err := controller.Start()

	log.Println("Starting controller", controller.Process.Pid)

	if err != nil {
		log.Fatal(err)
	}

	go func() {
		defer wg.Done()
		err := controller.Wait()
		if err != nil {
			log.Println("Controller", err)
		}
		log.Println("Controller exited")
	}()

	cmds := make([]*exec.Cmd, 0, NUM_AGENT)

	//start agents.
	for i := 0; i < NUM_AGENT; i++ {
		gid := i + 1
		agentCfg := utils.Format(AGENT_TMP, map[string]interface{}{
			"gid":   gid,
			"roles": AGENT_ROLES[gid],
		})

		agentCfgPath := fmt.Sprintf("/tmp/ag-%d.toml", gid)

		ioutil.WriteFile(agentCfgPath, []byte(agentCfg), 0644)

		agent := exec.Command(agentPath, "-c", agentCfgPath)
		cmds = append(cmds, agent)

		err := agent.Start()
		log.Println("Starting agent", i, agent.Process.Pid)
		go func() {
			defer wg.Done()
			err := agent.Wait()
			if err != nil {
				log.Println("Agent", gid, err)
			}
		}()

		if err != nil {
			log.Fatal(err)
		}
	}

	log.Println("Starting tests")
	code := m.Run()

	log.Println("Cleaning up")

	//controller.Process.Kill()
	syscall.Kill(controller.Process.Pid, syscall.SIGKILL)
	for _, cmd := range cmds {
		//cmd.Process.Kill()
		syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
	}

	wg.Wait()

	os.Exit(code)
}

func TestPing(t *testing.T) {
	clt := client.New("localhost:6379", "")
	for gid := 1; gid <= 2; gid++ {
		cmd := &client.Command{
			Id:  "test-ping",
			Gid: gid,
			Nid: 1,
			Cmd: "ping",
		}

		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}

		result, err := ref.Result(5)
		if err != nil {
			t.Fatal(err)
		}

		if result.Data != `"pong"` {
			t.Fatalf("Invalid response from agent %d", gid)
		}
	}
}

func TestParallelExecution(t *testing.T) {
	clt := client.New("localhost:6379", "")

	args := client.NewDefaultRunArgs()
	args[client.ARG_NAME] = "sleep"
	args[client.ARG_CMD_ARGS] = []string{"1"}

	gid := 1
	cmd := &client.Command{
		Gid:  gid,
		Nid:  1,
		Cmd:  "execute",
		Args: args,
	}

	count := 10
	refs := make([]*client.CommandReference, 0, count)

	for i := 0; i < count; i++ {
		cmd.Id = fmt.Sprintf("sleep-%d", i)
		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ref)
	}

	//get results
	st := 0
	for i, ref := range refs {
		result, err := ref.Result(10)
		if err != nil {
			t.Fatal(err)
		}

		if st == 0 {
			st = result.Starttime
		} else {
			if result.Starttime-st > 1000 {
				t.Fatal("Sleep jobs startime are not overlapping for job", i)
			}
		}

	}
}

func TestSerialExecution(t *testing.T) {
	clt := client.New("localhost:6379", "")

	args := client.NewDefaultRunArgs()
	args[client.ARG_NAME] = "sleep"
	args[client.ARG_CMD_ARGS] = []string{"1"}
	args[client.ARG_QUEUE] = "sleep-queue"

	gid := 1
	cmd := &client.Command{
		Gid:  gid,
		Nid:  1,
		Cmd:  "execute",
		Args: args,
	}

	count := 10
	refs := make([]*client.CommandReference, 0, count)

	for i := 0; i < count; i++ {
		cmd.Id = fmt.Sprintf("sleep-%d", i)
		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ref)
	}

	//get results
	st := 0
	for i, ref := range refs {
		result, err := ref.Result(10)
		if err != nil {
			t.Fatal(err)
		}

		if st != 0 {
			if result.Starttime-st < 1000 {
				t.Fatal("Sleep jobs startime are overlapping for job", i)
			}
		}

		st = result.Starttime
	}
}

func TestWrongCmd(t *testing.T) {
	clt := client.New("localhost:6379", "")
	for gid := 1; gid <= 2; gid++ {
		cmd := &client.Command{
			Id:  "test-ping",
			Gid: gid,
			Nid: 1,
			Cmd: "unknown",
		}

		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}

		result, err := ref.Result(5)
		if err != nil {
			t.Fatal(err)
		}

		if result.State != "UNKNOWN_CMD" {
			t.Fatalf("Invalid response from agent %d", gid)
		}
	}
}

func TestMaxTime(t *testing.T) {
	clt := client.New("localhost:6379", "")

	args := client.NewDefaultRunArgs()
	args[client.ARG_NAME] = "sleep"
	args[client.ARG_CMD_ARGS] = []string{"5"}
	args[client.ARG_MAX_TIME] = 1

	gid := 1
	cmd := &client.Command{
		Id:   "max-time",
		Gid:  gid,
		Nid:  1,
		Cmd:  "execute",
		Args: args,
	}

	ref, err := clt.Run(cmd)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ref.Result(10)
	if err != nil {
		t.Fatal(err)
	}

	if result.State != "TIMEOUT" {
		t.Fatal("Process state != KILLED")
	}
}

func TestRolesDistributed(t *testing.T) {
	clt := client.New("localhost:6379", "")
	count := 10

	counter := make(map[int]int)

	for i := 0; i < count; i++ {
		cmd := &client.Command{
			Id:   fmt.Sprintf("test-ping-%d", i),
			Gid:  0,
			Nid:  0,
			Cmd:  "ping",
			Role: "cpu",
		}

		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}

		result, err := ref.Result(1)
		if err != nil {
			t.Fatal(err)
		}

		counter[result.Gid] = counter[result.Gid] + 1
	}

	if len(counter) != NUM_AGENT {
		t.Fatal("Role execution didn't distribute the task as expected")
	}

	log.Println("Role CPU counters", counter)
}

func TestRolesSingle(t *testing.T) {
	clt := client.New("localhost:6379", "")
	count := 10

	counter := make(map[int]int)

	for i := 0; i < count; i++ {
		cmd := &client.Command{
			Id:   fmt.Sprintf("test-ping-%d", i),
			Gid:  0,
			Nid:  0,
			Cmd:  "ping",
			Role: "storage",
		}

		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}

		result, err := ref.Result(1)
		if err != nil {
			t.Fatal(err)
		}

		counter[result.Gid] = counter[result.Gid] + 1
	}

	if len(counter) != 1 {
		t.Fatal("Role execution didn't distribute the task as expected")
	}

	log.Println("Role CPU counters", counter)
}

func TestRolesDistributedSingleGrid(t *testing.T) {
	clt := client.New("localhost:6379", "")
	count := 10

	for i := 0; i < count; i++ {
		cmd := &client.Command{
			Id:   fmt.Sprintf("test-ping-%d", i),
			Gid:  1,
			Nid:  0,
			Cmd:  "ping",
			Role: "cpu",
		}

		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}

		result, err := ref.Result(1)
		if err != nil {
			t.Fatal(err)
		}

		if result.Gid != 1 {
			t.Fatal("Expected GID to be 1")
		}
	}

}

func TestPortForwarding(t *testing.T) {
	clt := client.New("localhost:6379", "")

	gid := 1
	cmd := &client.Command{
		Id:   "port-forward-open",
		Gid:  gid,
		Nid:  1,
		Cmd:  "hubble_open_tunnel",
		Data: `{"local": 9979, "gateway": "2.1", "ip": "127.0.0.1", "remote": 6379}`,
	}

	ref, err := clt.Run(cmd)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ref.Result(10)
	if err != nil {
		t.Fatal(err)
	}

	if result.State != "SUCCESS" {
		t.Fatal("Failed to open tunnel", result.Data)
	}

	//create test clt on new open port
	tunnel_clt := client.New("localhost:9979", "")

	ping := &client.Command{
		Id:   "tunnel-ping",
		Cmd:  "ping",
		Role: "cpu",
	}

	pref, err := tunnel_clt.Run(ping)
	if err != nil {
		t.Fatal("Failed to send command over tunneled connection")
	}

	pong, err := pref.Result(10)
	if err != nil {
		t.Fatal("Failed to retrieve result over tunneled connection")
	}

	if pong.State != "SUCCESS" {
		t.Fatal("Failed to ping/pong over tunneled connection")
	}

	//Closing the tunnel
	cmd = &client.Command{
		Id:   "port-forward-close",
		Gid:  gid,
		Nid:  1,
		Cmd:  "hubble_close_tunnel",
		Data: `{"local": 9979, "gateway": "2.1", "ip": "127.0.0.1", "remote": 6379}`,
	}

	if ref, err := clt.Run(cmd); err == nil {
		ref.Result(10)
	} else {
		t.Fatal("Failed to close the tunnel")
	}

	//try using the tunnel clt
	if ref, err := tunnel_clt.Run(ping); err == nil {
		ref.Result(10)
		t.Fatal("Expected an error, instead got success")
	}
}
