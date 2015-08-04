package agent

import (
	"fmt"
	_ "github.com/Jumpscale/jsagent"
	"github.com/Jumpscale/jsagent/agent/lib/utils"
	_ "github.com/Jumpscale/jsagentcontroller"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"testing"
)

const (
	NUM_AGENT = 2
)

var AGENT_TMP string = `
[main]
gid = 1
nid = {nid}
max_jobs = 100
message_id_file = "./.mid"
history_file = "./.history"
roles = []

[controllers]
    [controllers.main]
    url = "http://localhost:8966"
`

func TestMain(m *testing.M) {
	//start ac, and 2 agents.
	goPath := os.Getenv("GOPATH")
	agentPath := path.Join(goPath, "src", "github.com", "Jumpscale", "jsagent")
	controllerPath := path.Join(goPath, "src", "github.com", "Jumpscale", "jsagentcontroller")

	//start controller.
	controller := exec.Command("go", "run", "main.go", "-c", "agentcontroller.toml")
	controller.Dir = controllerPath
	err := controller.Start()

	if err != nil {
		log.Fatal(err)
	}

	//start agents.
	for i := 0; i < NUM_AGENT; i++ {
		nid := i + 1
		agentCfg := utils.Format(AGENT_TMP, map[string]interface{}{
			"nid": nid,
		})

		agentCfgPath := fmt.Sprintf("/tmp/ag-%d.toml", nid)

		ioutil.WriteFile(agentCfgPath, []byte(agentCfg), 644)

		agent := exec.Command("go", "run", "superagent.go", "-c", agentCfgPath)
		agent.Dir = agentPath
		err := agent.Start()
		if err != nil {
			log.Fatal(err)
		}
	}

	os.Exit(m.Run())
}
