package containers

import (
	"fmt"
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"os"
	"path"
	"syscall"
)

type hooks struct {
	args   *ContainerCreateArguments
	root   string
	coreID string

	onPID  pm.RunnerHook
	onExit pm.RunnerHook

	pid int
}

func newHook(args *ContainerCreateArguments, root, coreID string) *hooks {
	h := &hooks{
		args:   args,
		root:   root,
		coreID: coreID,
	}

	h.onPID = &pm.PIDHook{
		Action: h.onpid,
	}

	h.onExit = &pm.ExitHook{
		Action: h.onexit,
	}

	return h
}

func (h *hooks) onpid(pid int) {
	h.pid = pid
	h.postStart()
}

func (h *hooks) onexit(state bool) {
	log.Debugf("Container %s exited with state %v", h.coreID, state)
	h.cleanup()
}

func (h *hooks) cleanup() {
	redisSocketTarget := path.Join(h.root, "redis.socket")
	coreXTarget := path.Join(h.root, coreXBinaryName)

	pm.GetManager().Kill(fmt.Sprintf("net-%s", h.coreID))

	if h.pid > 0 {
		targetNs := fmt.Sprintf("/run/netns/%s", h.coreID)

		if err := syscall.Unmount(targetNs, 0); err != nil {
			log.Errorf("Failed to unmount %s: %s", targetNs, err)
		}
		os.RemoveAll(targetNs)
	}

	for _, guest := range h.args.Mount {
		target := path.Join(h.root, guest)
		if err := syscall.Unmount(target, syscall.MNT_FORCE); err != nil {
			log.Errorf("Failed to unmount %s: %s", target, err)
		}
	}

	if err := syscall.Unmount(redisSocketTarget, syscall.MNT_FORCE); err != nil {
		log.Errorf("Failed to unmount %s: %s", redisSocketTarget, err)
	}

	if err := syscall.Unmount(coreXTarget, syscall.MNT_FORCE); err != nil {
		log.Errorf("Failed to unmount %s: %s", coreXTarget, err)
	}

	if err := syscall.Unmount(h.root, syscall.MNT_FORCE); err != nil {
		log.Errorf("Failed to unmount %s: %s", h.root, err)
	}
}

func (h *hooks) zeroTier(netID string) error {
	sourceNs := fmt.Sprintf("/proc/%d/ns/net", h.pid)
	targetNs := fmt.Sprintf("/run/netns/%s", h.coreID)

	if f, err := os.Create(targetNs); err == nil {
		f.Close()
	}

	if err := syscall.Mount(sourceNs, targetNs, "", syscall.MS_BIND, ""); err != nil {
		return err
	}

	args := map[string]interface{}{
		"netns":    h.coreID,
		"zerotier": netID,
	}

	netcmd := core.Command{
		ID:        fmt.Sprintf("net-%s", h.coreID),
		Command:   "zerotier",
		Arguments: core.MustArguments(args),
	}

	_, err := pm.GetManager().RunCmd(&netcmd)
	return err
}

func (h *hooks) postStart() error {
	if h.args.Network.ZeroTier != "" {
		log.Debugf("Joining zerotier networ '%s'", h.args.Network.ZeroTier)
		if err := h.zeroTier(h.args.Network.ZeroTier); err != nil {
			return err
		}
	}

	return nil
}
