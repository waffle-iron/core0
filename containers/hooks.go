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
	args      *ContainerCreateArguments
	root      string
	container uint64

	onPID  pm.RunnerHook
	onExit pm.RunnerHook

	pid int
}

func newHook(args *ContainerCreateArguments, root string, container uint64) *hooks {
	h := &hooks{
		args:      args,
		root:      root,
		container: container,
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
	log.Debugf("Container %s exited with state %v", h.container, state)
	h.cleanup()
}

func (h *hooks) cleanup() {
	redisSocketTarget := path.Join(h.root, "redis.socket")
	coreXTarget := path.Join(h.root, coreXBinaryName)

	pm.GetManager().Kill(fmt.Sprintf("net-%v", h.container))

	if h.pid > 0 {
		targetNs := fmt.Sprintf("/run/netns/%v", h.container)

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

	//remove bridge links
	for _, bridge := range h.args.Network.Bridge {
		h.unbridge(bridge)
	}
}

func (h *hooks) namespace() error {
	sourceNs := fmt.Sprintf("/proc/%d/ns/net", h.pid)
	os.MkdirAll("/run/netns", 0755)
	targetNs := fmt.Sprintf("/run/netns/%v", h.container)

	if f, err := os.Create(targetNs); err == nil {
		f.Close()
	}

	if err := syscall.Mount(sourceNs, targetNs, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("namespace mount: %s", err)
	}

	return nil
}

func (h *hooks) zeroTier(netID string) error {
	args := map[string]interface{}{
		"netns":    h.container,
		"zerotier": netID,
	}

	netcmd := core.Command{
		ID:        fmt.Sprintf("net-%v", h.container),
		Command:   "zerotier",
		Arguments: core.MustArguments(args),
	}

	_, err := pm.GetManager().RunCmd(&netcmd)
	return err
}

func (h *hooks) unbridge(bridge ContainerBridgeSettings) error {
	name := fmt.Sprintf("%s-%v", bridge.Name(), h.container)

	link, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}

	return netlink.LinkDel(link)
}

func (h *hooks) bridge(index int, bridge ContainerBridgeSettings) error {
	link, err := netlink.LinkByName(bridge.Name())
	if err != nil {
		return err
	}

	if link.Type() != "bridge" {
		return fmt.Errorf("'%s' is not a bridge", link.Attrs().Name)
	}

	name := fmt.Sprintf("%s-%v", bridge.Name(), h.container)
	peerName := fmt.Sprintf("%s-%v-eth%d", bridge.Name(), h.container, index)

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:        name,
			Flags:       net.FlagUp,
			MTU:         1500,
			TxQLen:      1000,
			MasterIndex: link.Attrs().Index,
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

	dev := fmt.Sprintf("eth%d", index)

	cmd := &core.Command{
		ID:      uuid.New(),
		Command: process.CommandSystem,
		Arguments: core.MustArguments(
			process.SystemCommandArguments{
				Name: "ip",
				Args: []string{"netns", "exec", fmt.Sprintf("%v", h.container), "ip", "link", "set", peerName, "name", dev},
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

	//setting up bridged networking
	switch bridge.Setup() {
	case "":
	case "none":
	case "dhcp":
		//start a dhcpc inside the container.
		dhcpc := &core.Command{
			ID:      uuid.New(),
			Command: cmdContainerDispatch,
			Arguments: core.MustArguments(
				ContainerDispatchArguments{
					Container: h.container,
					Command: core.Command{
						ID:      "dhcpc",
						Command: process.CommandSystem,
						Arguments: core.MustArguments(
							process.SystemCommandArguments{
								Name: "udhcpc",
								Args: []string{
									"-f",
									"-i", dev,
									"-s", "/usr/share/udhcp/simple.script",
								},
							},
						),
					},
				},
			),
		}
		pm.GetManager().RunCmd(dhcpc)
	default:
		//set static ip
		if _, _, err := net.ParseCIDR(bridge.Setup()); err != nil {
			return err
		}

		cmd := &core.Command{
			ID:      uuid.New(),
			Command: process.CommandSystem,
			Arguments: core.MustArguments(
				process.SystemCommandArguments{
					Name: "ip",
					Args: []string{"netns", "exec", fmt.Sprintf("%v", h.container), "ip", "address", "add", bridge.Setup(), "dev", dev},
				},
			),
		}
		pm.GetManager().RunCmd(cmd)
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
		log.Debugf("Connecting container to bridge '%s'", bridge)
		if err := h.bridge(i, bridge); err != nil {
			return err
		}
	}

	return nil
}
