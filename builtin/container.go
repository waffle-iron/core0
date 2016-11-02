package builtin

import (
	"encoding/json"
	"fmt"
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"
)

const (
	cmdContainerCreate = "corex.create"

	coreXBinaryName = "coreX"

	redisSocket = "/var/run/redis.socket"
)

type containerManager struct {
	sequence uint64
	mutex    sync.Mutex
}

func init() {
	containerMgr := &containerManager{}

	pm.CmdMap[cmdContainerCreate] = process.NewInternalProcessFactory(containerMgr.create)
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

func (m *containerManager) bind(root string) error {
	redisSocketTarget := path.Join(root, redisSocket)
	coreXTarget := path.Join(root, coreXBinaryName)

	os.Create(redisSocketTarget)
	os.Create(coreXTarget)

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

func (m *containerManager) unbind(root string) {
	redisSocketTarget := path.Join(root, redisSocket)
	coreXTarget := path.Join(root, coreXBinaryName)

	if err := syscall.Unmount(redisSocketTarget, 0); err != nil {
		log.Errorf("Failed to unmount %s: %s", redisSocketTarget, err)
	}

	if err := syscall.Unmount(coreXTarget, 0); err != nil {
		log.Errorf("Failed to unmount %s: %s", coreXTarget, err)
	}
}

func (m *containerManager) create(cmd *core.Command) (interface{}, error) {
	var args ContainerCreateArguments
	if err := json.Unmarshal(cmd.Arguments, &args); err != nil {
		return nil, err
	}

	//TODO: this need to be replaced by a plist url or similar
	if args.RootMount == "" {
		return nil, fmt.Errorf("invalid root_mount")
	}

	id := m.getNextSequence()
	coreID := fmt.Sprintf("core-%d", id)

	if err := m.bind(args.RootMount); err != nil {
		m.unbind(args.RootMount)
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
				},
				Env: map[string]string{
					"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				},
			},
		),
	}

	cleanup := func(id uint64, root string) pm.RunnerHook {
		return func(state bool) {
			log.Debugf("Container %d exited with state %v", id, state)
			m.unbind(root)
		}
	}(id, args.RootMount)

	_, err := mgr.NewRunner(extCmd, process.NewContainerProcess, -1, cleanup)
	if err != nil {
		return nil, err
	}

	return id, nil
}
