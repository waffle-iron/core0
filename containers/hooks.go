package containers

import (
	"fmt"
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
	"github.com/pborman/uuid"
	"github.com/vishvananda/netlink"
	"net"
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
	if err := h.postStart(); err != nil {
		log.Errorf("Container post start error: %s", err)
		//TODO. Should we shut the container down?
	}
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

func (h *hooks) namespace() error {
	sourceNs := fmt.Sprintf("/proc/%d/ns/net", h.pid)
	targetNs := fmt.Sprintf("/run/netns/%s", h.coreID)

	if f, err := os.Create(targetNs); err == nil {
		f.Close()
	}

	if err := syscall.Mount(sourceNs, targetNs, "", syscall.MS_BIND, ""); err != nil {
		return err
	}

	return nil
}

func (h *hooks) zeroTier(netID string) error {
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

func (h *hooks) bridge(index int, bridge string) error {
	link, err := netlink.LinkByName(bridge)
	if err != nil {
		return err
	}

	br, ok := link.(*netlink.Bridge)

	if link.Type() != "bridge" || !ok {
		return fmt.Errorf("'%s' is not a bridge", link.Attrs().Name)
	}

	name := fmt.Sprintf("%s-%s", bridge, h.coreID)
	peerName := fmt.Sprintf("%s-%s-eth%d", bridge, h.coreID, index)

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:  name,
			Flags: net.FlagUp,
		},
		PeerName: peerName,
	}

	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("create link: %s", err)
	}

	peer, err := netlink.LinkByName(peerName)
	if err != nil {
		return fmt.Errorf("get peer: %s", err)
	}

	if err := netlink.LinkSetMaster(veth, br); err != nil {
		return fmt.Errorf("set master: %s", err)
	}

	if err := netlink.LinkSetUp(peer); err != nil {
		return fmt.Errorf("set up: %s", err)
	}

	if err := netlink.LinkSetNsPid(peer, h.pid); err != nil {
		return fmt.Errorf("set ns pid: %s", err)
	}

	//TODO: this doesn't work after moving the device to the NS.
	//But we can't rename as well before joining the ns, otherwise we
	//can end up with conflicting name on the host namespace.
	//if err := netlink.LinkSetName(peer, fmt.Sprintf("eth%d", index)); err != nil {
	//	return fmt.Errorf("set link name: %s", err)
	//}

	cmd := &core.Command{
		ID:      uuid.New(),
		Command: process.CommandSystem,
		Arguments: core.MustArguments(
			process.SystemCommandArguments{
				Name: "ip",
				Args: []string{"netns", "exec", h.coreID, "ip", "link", "set", peerName, "name", fmt.Sprintf("eth%d", index)},
			},
		),
	}
	runner, err := pm.GetManager().RunCmd(cmd)

	if err != nil {
		return err
	}

	result := runner.Wait()
	if result.State != core.StateSuccess {
		return fmt.Errorf("failed to rename device: %s", result.Streams)
	}

	return nil
}

func (h *hooks) postStart() error {
	if err := h.namespace(); err != nil {
		return err
	}

	if h.args.Network.ZeroTier != "" {
		log.Debugf("Joining zerotier networ '%s'", h.args.Network.ZeroTier)
		if err := h.zeroTier(h.args.Network.ZeroTier); err != nil {
			return err
		}
	}

	for i, bridge := range h.args.Network.Bridge {
		log.Debugf("Connecting container to bridge '%s'", h.args.Network.Bridge)
		if err := h.bridge(i, bridge); err != nil {
			return err
		}
	}

	return nil
}
