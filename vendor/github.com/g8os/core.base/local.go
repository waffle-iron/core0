package core

import (
	"encoding/json"
	"fmt"
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/utils"
	"net"
	"os"
)

type Local struct {
	listener *net.UnixListener
}

type LocalCmd struct {
	Sync    bool            `json:"sync"`
	Content json.RawMessage `json:"content"`
}

type LocalResult struct {
	State  string          `json:"state"`
	Error  string          `json:"error,omitempty"`
	Result *core.JobResult `json:"result,omitempty"`
}

func NewLocal(s string) (*Local, error) {
	if utils.Exists(s) {
		os.Remove(s)
	}

	addr, err := net.ResolveUnixAddr("unix", s)
	if err != nil {
		return nil, err
	}
	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, err
	}
	return &Local{
		listener,
	}, nil
}

func (l *Local) server(con net.Conn) {
	//read command

	lresult := LocalResult{
		State: core.StateError,
	}

	defer func() {
		//send result
		m, _ := json.Marshal(&lresult)
		if _, err := con.Write(m); err != nil {
			log.Errorf("Failed to write response to local transport: %s", err)
		}
		con.Close()
	}()

	decoder := json.NewDecoder(con)
	var lcmd LocalCmd

	if err := decoder.Decode(&lcmd); err != nil {
		lresult.Error = fmt.Sprintf("Failed to decode message: %s", err)
		return
	}

	cmd, err := core.LoadCmd(lcmd.Content)
	if err != nil {
		lresult.Error = fmt.Sprintf("Failed to extract command: %s", err)
		return
	}

	runner, err := pm.GetManager().RunCmd(cmd)
	if err != nil {
		lresult.Error = fmt.Sprintf("Failed to get job runner for command(%s): %s", cmd.Command, err)
		return
	}

	go runner.Run()
	lresult.State = core.StateSuccess

	if lcmd.Sync {
		result := runner.Wait()
		lresult.Result = result
	}
}

func (l *Local) Serve() {
	defer l.listener.Close()
	for {
		con, err := l.listener.Accept()
		if err != nil {
			log.Errorf("local transport error: %s", err)
		}
		go l.server(con)
	}
}
