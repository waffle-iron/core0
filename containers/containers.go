package containers

import (
	"encoding/json"
	"fmt"
	base "github.com/g8os/core.base"
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
	"github.com/g8os/core.base/utils"
	"github.com/garyburd/redigo/redis"
	"github.com/op/go-logging"
	"github.com/pborman/uuid"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"
	"time"
)

const (
	cmdContainerCreate   = "corex.create"
	cmdContainerList     = "corex.list"
	cmdContainerDispatch = "corex.dispatch"
	coreXResponseQueue   = "corex:results"

	coreXBinaryName = "coreX"

	redisSocket = "/var/run/redis.socket"
)

var (
	log = logging.MustGetLogger("containers")
)

type containerManager struct {
	sequence uint64
	mutex    sync.Mutex

	pool   *redis.Pool
	ensure sync.Once

	sinks map[string]base.SinkClient
}

/*
WARNING:
	Code here assumes that redis-server is started by core0 by the configuration files. If it wasn't started or failed
	to start, commands like core.create, core.dispatch, etc... will fail.
TODO:
	May be make redis-server start part of the bootstrap process without the need to depend on external configuration
	to run it.
*/

func Containers(sinks map[string]base.SinkClient) {
	containerMgr := &containerManager{
		pool:  utils.NewRedisPool("unix", redisSocket, ""),
		sinks: sinks,
	}

	pm.CmdMap[cmdContainerCreate] = process.NewInternalProcessFactory(containerMgr.create)
	pm.CmdMap[cmdContainerList] = process.NewInternalProcessFactory(containerMgr.list)
	pm.CmdMap[cmdContainerDispatch] = process.NewInternalProcessFactory(containerMgr.dispatch)

	go containerMgr.startForwarder()
}

func (m *containerManager) forwardNext() error {
	db := m.pool.Get()
	defer db.Close()

	payload, err := redis.ByteSlices(db.Do("BLPOP", coreXResponseQueue, 0))
	if err != nil {
		return err
	}

	var result core.JobResult
	if err := json.Unmarshal(payload[1], &result); err != nil {
		log.Errorf("Failed to load command: %s", err)
		return nil //no wait.
	}

	//use command tags for routing.
	if sink, ok := m.sinks[result.Tags]; ok {
		log.Debugf("Forwarding job result to %s", result.Tags)
		return sink.Respond(&result)
	} else {
		log.Warningf("Received a corex result for an unknown sink: %s", result.Tags)
	}

	return nil
}

func (m *containerManager) startForwarder() {
	log.Debugf("Start container results forwarder")
	for {
		if err := m.forwardNext(); err != nil {
			log.Warning("Failed to forward command result: %s", err)
			time.Sleep(2 * time.Second)
		}
	}
}

func (m *containerManager) getNextSequence() uint64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.sequence += 1
	return m.sequence
}

type ContainerCreateArguments struct {
	RootMount string `json:"root_mount"`
}

func (m *containerManager) preBind(args *ContainerCreateArguments) error {
	redisSocketTarget := path.Join(args.RootMount, redisSocket)
	coreXTarget := path.Join(args.RootMount, coreXBinaryName)

	if f, err := os.Create(redisSocketTarget); err == nil {
		f.Close()
	}

	if f, err := os.Create(coreXTarget); err == nil {
		f.Close()
	}

	if err := syscall.Mount(redisSocket, redisSocketTarget, "", syscall.MS_BIND, ""); err != nil {
		return err
	}

	coreXSrc, err := exec.LookPath(coreXBinaryName)
	if err != nil {
		return err
	}

	if err := syscall.Mount(coreXSrc, coreXTarget, "", syscall.MS_BIND, ""); err != nil {
		return err
	}

	return nil
}

func (m *containerManager) postBind(coreID string, pid int) error {
	sourceNs := fmt.Sprintf("/proc/%d/ns/net", pid)
	targetNs := fmt.Sprintf("/run/netns/%s", coreID)

	if f, err := os.Create(targetNs); err == nil {
		f.Close()
	}

	return syscall.Mount(sourceNs, targetNs, "", syscall.MS_BIND, "")
}

func (m *containerManager) unbind(coreID string, pid int, args *ContainerCreateArguments) {
	redisSocketTarget := path.Join(args.RootMount, redisSocket)
	coreXTarget := path.Join(args.RootMount, coreXBinaryName)

	if pid > 0 {
		targetNs := fmt.Sprintf("/run/netns/%s", coreID)

		if err := syscall.Unmount(targetNs, 0); err != nil {
			log.Errorf("Failed to unmount %s: %s", targetNs, err)
		}
		os.RemoveAll(targetNs)
	}

	if err := syscall.Unmount(redisSocketTarget, 0); err != nil {
		log.Errorf("Failed to unmount %s: %s", redisSocketTarget, err)
	}

	if err := syscall.Unmount(coreXTarget, 0); err != nil {
		log.Errorf("Failed to unmount %s: %s", coreXTarget, err)
	}
}

func (m *containerManager) create(cmd *core.Command) (interface{}, error) {
	var args ContainerCreateArguments
	if err := json.Unmarshal(*cmd.Arguments, &args); err != nil {
		return nil, err
	}

	//TODO: this need to be replaced by a plist url or similar
	if args.RootMount == "" {
		return nil, fmt.Errorf("invalid root_mount")
	}

	id := m.getNextSequence()
	coreID := fmt.Sprintf("core-%d", id)

	if err := m.preBind(&args); err != nil {
		m.unbind(coreID, 0, &args)
		return nil, err
	}

	mgr := pm.GetManager()
	extCmd := &core.Command{
		ID:    coreID,
		Route: cmd.Route,
		Arguments: core.MustArguments(
			process.ContainerCommandArguments{
				Name:   "/coreX",
				Chroot: args.RootMount,
				Dir:    "/",
				Args: []string{
					"-core-id", fmt.Sprintf("%d", id),
					"-redis-socket", "/var/run/redis.socket",
					"-reply-to", coreXResponseQueue,
				},
				Env: map[string]string{
					"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				},
			},
		),
	}

	hook := hooks{
		mgr:    m,
		args:   &args,
		coreID: coreID,
	}

	onExit := &pm.ExitHook{
		Action: hook.onExit,
	}

	onPID := &pm.PIDHook{
		Action: hook.onPID,
	}

	_, err := mgr.NewRunner(extCmd, process.NewContainerProcess, onPID, onExit)
	if err != nil {
		return nil, err
	}

	return id, nil
}

func (m *containerManager) list(cmd *core.Command) (interface{}, error) {
	containers := make(map[string]*process.ProcessStats)

	for name, runner := range pm.GetManager().Runners() {
		var id uint64
		if n, err := fmt.Sscanf(name, "core-%d", &id); err != nil || n != 1 {
			continue
		}
		ps := runner.Process()
		if ps != nil {
			state := ps.GetStats()
			state.Cmd = nil
			containers[name] = state
		} else {
			containers[name] = nil
		}
	}

	return containers, nil
}

type ContainerDispatchArguments struct {
	Container uint64       `json:"container"`
	Command   core.Command `json:"command"`
}

func (m *containerManager) getCoreXQueue(id uint64) string {
	return fmt.Sprintf("core:default:core-%v", id)
}

func (m *containerManager) dispatch(cmd *core.Command) (interface{}, error) {
	var args ContainerDispatchArguments
	if err := json.Unmarshal(*cmd.Arguments, &args); err != nil {
		return nil, err
	}

	if args.Container <= 0 {
		return nil, fmt.Errorf("invalid container id")
	}

	if _, ok := pm.GetManager().Runners()[fmt.Sprintf("core-%d", args.Container)]; !ok {
		return nil, fmt.Errorf("container does not exist")
	}

	id := uuid.New()
	args.Command.ID = id
	args.Command.Tags = string(cmd.Route)

	db := m.pool.Get()
	defer db.Close()

	data, err := json.Marshal(args.Command)
	if err != nil {
		return nil, err
	}

	_, err = db.Do("RPUSH", m.getCoreXQueue(args.Container), string(data))

	return id, err
}
