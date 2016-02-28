package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/g8os/core/agent/lib/utils"
	"github.com/g8os/core/tests/client"
)

const (
	NumAgents = 2
)

var AgentCfgTmp = `
[main]
gid = {gid}
nid = 1
max_jobs = 100
message_ID_file = "./.mid"
history_file = "./.history"
roles = {roles}

[controllers]
    [controllers.main]
    url = "http://localhost:1221"

[extensions]
	[extensions.bash]
	    binary = "bash"
	    args = ['-c', 'T=$(mktemp) && cat > $T && chmod +x $T && bash -c $T; EXIT=$?; rm -rf $T; exit $EXIT']
`

var ControllerCfgTmp = `
[main]
redis_host =  "127.0.0.1:6379"
redis_password = ""

[[listen]]
    address = ":1221"

[influxdb]
host = "127.0.0.1:8086"
db   = "agentcontroller"
user = "ac"
password = "acctrl"


`

var AgentRoles = map[int]string{
	1: `["cpu"]`,
	2: `["cpu", "storage"]`,
}

func TestMain(m *testing.M) {
	//start ac, and 2 agents.
	goPath := os.Getenv("GOPATH")
	agentPath := path.Join(goPath, "bin", "agent8")
	controllerPath := path.Join(goPath, "bin", "agentcontroller8")

	{
		//build controller
		log.Println("Building agent controller")
		cmd := exec.Command("go", "install", "github.com/Jumpscale/agentcontroller8")
		if err := cmd.Run(); err != nil {
			log.Fatal("Failed to buld controller", err)
		}
	}

	{
		//build agent
		log.Println("Building agent")
		cmd := exec.Command("go", "install", "github.com/g8os/core")
		if err := cmd.Run(); err != nil {
			log.Fatal("Failed to build agent", err)
		}
	}

	//start controller.
	var wg sync.WaitGroup
	wg.Add(NumAgents + 1)

	ctrlCfg := utils.Format(ControllerCfgTmp, map[string]interface{}{})

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

	cmds := make([]*exec.Cmd, 0, NumAgents)

	//start agents.
	for i := 0; i < NumAgents; i++ {
		gid := i + 1
		agentCfg := utils.Format(AgentCfgTmp, map[string]interface{}{
			"gid":   gid,
			"roles": AgentRoles[gid],
		})

		agentCfgPath := fmt.Sprintf("/tmp/ag-%d.toml", gid)

		ioutil.WriteFile(agentCfgPath, []byte(agentCfg), 0644)

		agent := exec.Command(agentPath, "-c", agentCfgPath)
		cmds = append(cmds, agent)

		reader, err := agent.StderrPipe()
		if err != nil {
			log.Fatal(err)
		}

		err = agent.Start()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Starting agent", i, agent.Process.Pid)
		go func(gid int, agent *exec.Cmd, reader io.ReadCloser) {
			defer wg.Done()
			dst, err := os.Create(fmt.Sprintf("/tmp/agent-%d.log", gid))
			if err != nil {
				log.Fatal(err)
			}
			log.Println("Copying log stream")
			copied, err := io.Copy(dst, reader)
			if err != nil {
				log.Fatal(err)
			}
			reader.Close()
			dst.Close()
			log.Println("Log", copied)
			log.Println("Waiting for agent to exit")
			err = agent.Wait()
			if err != nil {
				log.Println("Agent", gid, err)
			}
		}(gid, agent, reader)

		if err != nil {
			log.Fatal(err)
		}
	}

	log.Println("Starting tests")
	time.Sleep(5 * time.Second)
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
			Gid: gid,
			Nid: 1,
			Cmd: "ping",
		}

		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}

		result, err := ref.GetNextResult(5)
		if err != nil {
			t.Fatal(err)
		}

		if result.Data != `"pong"` {
			t.Fatalf("Invalid response from agent %d", gid)
		}
	}
}

func TestKill(t *testing.T) {
	clt := client.New("localhost:6379", "")

	args := client.NewDefaultRunArgs()
	args[client.ArgName] = "sleep"
	args[client.ArgCmdArguments] = []string{"60"}

	cmd := &client.Command{
		Gid:  1,
		Nid:  1,
		Cmd:  "execute",
		Args: args,
	}

	ref, err := clt.Run(cmd)
	if err != nil {
		t.Fatal(err)
	}

	jobs, err := ref.GetJobs(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatal("Invalid number of jobs")
	}

	job := jobs[0]

	killCmd := &client.Command{
		Gid:  1,
		Nid:  1,
		Cmd:  "kill",
		Data: fmt.Sprintf("{\"id\": \"%s\"}", ref.ID),
	}

	killRef, err := clt.Run(killCmd)
	if err != nil {
		t.Fatal(err)
	}

	killRes, err := killRef.GetNextResult(5)
	if err != nil {
		t.Fatal("Failed to kill job", err)
	}

	if killRes.State != "SUCCESS" {
		t.Fatal("Kill job failed with error", killRes.Data)
	}

	err = job.Wait(5)
	if err != nil {
		t.Fatal("Failed while waiting for job", err)
	}

	if job.State != "KILLED" {
		t.Fatal("Expected a KILLED status, got", job.State, "instead")
	}
}

func TestStatsTrackingWithChildren(t *testing.T) {
	//runs a script that forks, and then the child process
	//start eating up memory. The main process will print the PID of the
	//child in the expected format.
	//Also the main process will wait for the child process to exit. before it
	//terminates so we have enough time to query the memory multiple times.

	script := `
	set -e

	function eatmemory {
	        echo "Start allocating memory..."
	        for index in $(seq 10000); do
	                mem[$index]=$(seq -w -s '' 1000)
	        done
	        echo "Memory allocation done..."
	}

	eatmemory &

	PID=$(jobs -l|awk '{print $2}')

	echo "101::$PID"
	echo "Waiting for $PID"
	wait $PID
	echo "Done"

	exit 0
	`

	clt := client.New("localhost:6379", "")
	cmd := &client.Command{
		Gid:  1,
		Nid:  1,
		Cmd:  "bash",
		Data: script,
	}

	ref, err := clt.Run(cmd)
	if err != nil {
		t.Fatal(err)
	}

	jobs, err := ref.GetJobs(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatal("Expected one job")
	}

	job := jobs[0]

	var result struct {
		VMS int `json:"vms"`
	}

	wait := make(chan int)
	go func() {
		err := job.Wait(0)
		if err != nil {
			t.Fatal(err)
		}
		wait <- 1
	}()

	var readings = make([]float64, 0)
loop:
	for {
		statsCmd := &client.Command{
			Gid:  1,
			Nid:  1,
			Cmd:  "get_process_stats",
			Data: fmt.Sprintf("{\"id\": \"%s\"}", ref.ID),
		}

		statsRef, err := clt.Run(statsCmd)
		stats, err := statsRef.GetNextResult(1)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(stats.Data), &result); err != nil {
			t.Fatal(err)
		}

		readings = append(readings, float64(result.VMS))

		select {
		case <-wait:
			break loop
		case <-time.After(time.Second):
		}
	}

	//check readings growth.
	growth := 0.0
	for i := 0; i < len(readings)-1; i++ {
		diff := readings[i+1] - readings[i]
		growth += diff
	}

	avg := growth / float64(len(readings))
	log.Println("Average memory growth is", avg)
	if avg <= 0 {
		t.Fatal("Memory doesn't grow as expected")
	}

	err = job.Wait(0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParallelExecution(t *testing.T) {
	clt := client.New("localhost:6379", "")

	args := client.NewDefaultRunArgs()
	args[client.ArgName] = "sleep"
	args[client.ArgCmdArguments] = []string{"1"}

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
		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ref)
	}

	//get results
	st := 0
	for i, ref := range refs {
		result, err := ref.GetNextResult(10)
		if err != nil {
			t.Fatal(err)
		}

		if st == 0 {
			st = result.StartTime
		} else {
			if result.StartTime-st > 1000 {
				t.Fatal("Sleep jobs startime are not overlapping for job", i)
			}
		}

	}
}

func TestSerialExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	clt := client.New("localhost:6379", "")

	args := client.NewDefaultRunArgs()
	args[client.ArgName] = "sleep"
	args[client.ArgCmdArguments] = []string{"1"}
	args[client.ArgQueue] = "sleep-queue"

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
		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ref)
	}

	//get results
	st := 0
	for i, ref := range refs {
		result, err := ref.GetNextResult(10)
		if err != nil {
			t.Fatal(err)
		}

		if st != 0 {
			if result.StartTime-st < 1000 {
				t.Fatal("Sleep jobs startime are overlapping for job", i)
			}
		}

		st = result.StartTime
	}
}

func TestWrongCmd(t *testing.T) {
	clt := client.New("localhost:6379", "")
	for gid := 1; gid <= 2; gid++ {
		cmd := &client.Command{
			Gid: gid,
			Nid: 1,
			Cmd: "unknown",
		}

		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}

		result, err := ref.GetNextResult(5)
		if err != nil {
			t.Fatal(err)
		}

		if result.State != "UNKNOWN_CMD" {
			t.Fatalf("Invalid response from agent %d", gid)
		}
	}
}

func TestMaxTime(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	clt := client.New("localhost:6379", "")

	args := client.NewDefaultRunArgs()
	args[client.ArgName] = "sleep"
	args[client.ArgCmdArguments] = []string{"5"}
	args[client.ArgMaxTime] = 1

	gid := 1
	cmd := &client.Command{
		Gid:  gid,
		Nid:  1,
		Cmd:  "execute",
		Args: args,
	}

	ref, err := clt.Run(cmd)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ref.GetNextResult(10)
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
			Gid:   0,
			Nid:   0,
			Cmd:   "ping",
			Roles: []string{"cpu"},
		}

		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}

		result, err := ref.GetNextResult(1)
		if err != nil {
			t.Fatal(err)
		}

		counter[result.Gid] = counter[result.Gid] + 1
	}

	if len(counter) != NumAgents {
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
			Gid:   0,
			Nid:   0,
			Cmd:   "ping",
			Roles: []string{"storage"},
		}

		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}

		result, err := ref.GetNextResult(1)
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
			Gid:   1,
			Nid:   0,
			Cmd:   "ping",
			Roles: []string{"cpu"},
		}

		ref, err := clt.Run(cmd)
		if err != nil {
			t.Fatal(err)
		}

		result, err := ref.GetNextResult(1)
		if err != nil {
			t.Fatal(err)
		}

		if result.Gid != 1 {
			t.Fatal("Expected GID to be 1")
		}
	}

}

func TestRoleFanout(t *testing.T) {
	clt := client.New("localhost:6379", "")

	cmd := &client.Command{
		Gid:    0,
		Nid:    0,
		Cmd:    "ping",
		Roles:  []string{"cpu"},
		Fanout: true,
	}

	ref, err := clt.Run(cmd)
	if err != nil {
		t.Fatal(err)
	}

	jobs, err := ref.GetJobs(1)
	if err != nil {
		t.Fatal(err)
	}

	if len(jobs) != NumAgents {
		t.Fatal("Invalid number of jobs ", len(jobs), " expected", NumAgents)
	}

	for _, job := range jobs {
		if err := job.Wait(1); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRoleFanoutAll(t *testing.T) {
	clt := client.New("localhost:6379", "")

	cmd := &client.Command{
		Gid:    0,
		Nid:    0,
		Cmd:    "ping",
		Roles:  []string{"*"},
		Fanout: true,
	}

	ref, err := clt.Run(cmd)
	if err != nil {
		t.Fatal(err)
	}

	//expecting NumAgents results
	jobs, err := ref.GetJobs(1)
	if err != nil {
		t.Fatal(err)
	}

	if len(jobs) != NumAgents {
		t.Fatal("Invalid number of jobs ", len(jobs), " expected", NumAgents)
	}

	for _, job := range jobs {
		if err := job.Wait(1); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRoleFanoutSingle(t *testing.T) {
	clt := client.New("localhost:6379", "")

	cmd := &client.Command{
		Gid:    0,
		Nid:    0,
		Cmd:    "ping",
		Roles:  []string{"storage"},
		Fanout: true,
	}

	ref, err := clt.Run(cmd)
	if err != nil {
		t.Fatal(err)
	}

	//expecting NumAgents results
	jobs, err := ref.GetJobs(1)
	if err != nil {
		t.Fatal(err)
	}

	if len(jobs) != 1 {
		t.Fatal("Invalid number of jobs '", len(jobs), "' expected 1")
	}

	job := jobs[0]
	err = job.Wait(1)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPortForwarding(t *testing.T) {
	clt := client.New("localhost:6379", "")

	gid := 1
	cmd := &client.Command{
		Gid:  gid,
		Nid:  1,
		Cmd:  "hubble_open_tunnel",
		Data: `{"local": 9979, "gateway": "2.1", "ip": "127.0.0.1", "remote": 6379}`,
	}

	ref, err := clt.Run(cmd)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ref.GetNextResult(10)
	if err != nil {
		t.Fatal(err)
	}

	if result.State != "SUCCESS" {
		t.Fatal("Failed to open tunnel", result.Data)
	}

	//create test client on new open port
	tunnelClient := client.New("localhost:9979", "")

	ping := &client.Command{
		Cmd:   "ping",
		Roles: []string{"cpu"},
	}

	pref, err := tunnelClient.Run(ping)
	if err != nil {
		t.Fatal("Failed to send command over tunneled connection")
	}

	pong, err := pref.GetNextResult(10)
	if err != nil {
		t.Fatal("Failed to retrieve result over tunneled connection")
	}

	if pong.State != "SUCCESS" {
		t.Fatal("Failed to ping/pong over tunneled connection")
	}

	//Closing the tunnel
	cmd = &client.Command{
		Gid:  gid,
		Nid:  1,
		Cmd:  "hubble_close_tunnel",
		Data: `{"local": 9979, "gateway": "2.1", "ip": "127.0.0.1", "remote": 6379}`,
	}

	if ref, err := clt.Run(cmd); err == nil {
		ref.GetNextResult(10)
	} else {
		t.Fatal("Failed to close the tunnel")
	}

	//try using the tunnel clt
	if ref, err := tunnelClient.Run(ping); err == nil {
		ref.GetNextResult(10)
		t.Fatal("Expected an error, instead got success")
	}
}

func TestRoutingWorngId(t *testing.T) {
	clt := client.New("localhost:6379", "")
	cmd := &client.Command{
		Gid: 1,
		Nid: 10,
		Cmd: "ping",
	}

	ref, err := clt.Run(cmd)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ref.GetNextResult(5)
	if err != nil {
		t.Fatal(err)
	}

	if result.State != "ERROR" {
		t.Fatalf("Invalid response from agent controller. Expected error")
	}

	if result.Data != "No matching connected agents found" {
		t.Fatal("Expecting, 'Agent is not alive!' message")
	}
}

func TestRoutingWrongRole(t *testing.T) {
	clt := client.New("localhost:6379", "")
	cmd := &client.Command{
		Gid:    0,
		Nid:    0,
		Cmd:    "ping",
		Roles:  []string{"unknown"},
		Fanout: false,
	}

	ref, err := clt.Run(cmd)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ref.GetNextResult(5)
	if err != nil {
		t.Fatal(err)
	}

	if result.State != "ERROR" {
		t.Fatalf("Invalid response from agent controller. Expected error")
	}

	if result.Data != "No matching connected agents found" {
		t.Fatal("Expecting, 'Agent is not alive!' message got ", result.Data, " instead")
	}
}

func TestRoutingWrongRoleWithFanout(t *testing.T) {
	clt := client.New("localhost:6379", "")
	cmd := &client.Command{
		Gid:    0,
		Nid:    0,
		Cmd:    "ping",
		Roles:  []string{"unknown"},
		Fanout: true,
	}

	ref, err := clt.Run(cmd)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ref.GetNextResult(5)
	if err != nil {
		t.Fatal(err)
	}

	if result.State != "ERROR" {
		t.Fatalf("Invalid response from agent controller. Expected error")
	}

	if result.Data != "No matching connected agents found" {
		t.Fatal("Expecting, 'Agent is not alive!' message got ", result.Data, " instead")
	}
}
