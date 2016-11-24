package builtin

import (
	"encoding/json"
	"fmt"
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
	"github.com/vishvananda/netlink"
	"net"
	"os"
)

func init() {
	pm.CmdMap["bridge.create"] = process.NewInternalProcessFactory(bridgeCreate)
	pm.CmdMap["bridge.list"] = process.NewInternalProcessFactory(bridgeList)
	pm.CmdMap["bridge.delete"] = process.NewInternalProcessFactory(bridgeDelete)
}

const (
	NoneBridgeNetworkMode    BridgeNetworkMode = ""
	DnsMasqBridgeNetworkMode BridgeNetworkMode = "dnsmasq"
	StaticBridgeNetworkMode  BridgeNetworkMode = "static"
)

type BridgeNetworkMode string

type NetworkStaticSettings struct {
	CIDR string `json:"cidr"`
}

type NetworkDnsMasqSettings struct {
	NetworkStaticSettings
	Start net.IP `json:"start"`
	End   net.IP `json:"end"`
}

type BridgeNetwork struct {
	Mode     BridgeNetworkMode `json:"mode"`
	Settings json.RawMessage   `json:"settings"`
}

type BridgeCreateArguments struct {
	Name      string        `json:"name"`
	HwAddress string        `json:"hwaddr"`
	Network   BridgeNetwork `json:"network"`
}

type BridgeDeleteArguments struct {
	Name string `json:"name"`
}

func bridgeStaticNetworking(bridge *netlink.Bridge, network *BridgeNetwork) error {
	var settings NetworkStaticSettings
	if err := json.Unmarshal(network.Settings, &settings); err != nil {
		return err
	}

	addr, err := netlink.ParseAddr(settings.CIDR)
	if err != nil {
		return err
	}

	return netlink.AddrAdd(bridge, addr)
}

func bridgeDnsMasqNetworking(bridge *netlink.Bridge, network *BridgeNetwork) error {
	var settings NetworkDnsMasqSettings
	if err := json.Unmarshal(network.Settings, &settings); err != nil {
		return err
	}

	os.MkdirAll("/var/run/dnsmasq", 0755)

	addr, err := netlink.ParseAddr(settings.CIDR)
	if err != nil {
		return err
	}

	if err := netlink.AddrAdd(bridge, addr); err != nil {
		return err
	}

	args := []string{
		"--no-hosts",
		"--keep-in-foreground",
		fmt.Sprintf("--pid-file=/var/run/dnsmasq/%s.pid", bridge.Name),
		fmt.Sprintf("--interface=%s", bridge.Name),
		fmt.Sprintf("--dhcp-range=%s,%s,%s,infinite", settings.Start, settings.End, net.IP(addr.Mask)),
	}

	cmd := &core.Command{
		ID:      fmt.Sprintf("dnsmasq-%s", bridge.Name),
		Command: process.CommandSystem,
		Arguments: core.MustArguments(
			process.SystemCommandArguments{
				Name: "dnsmasq",
				Args: args,
			},
		),
	}

	onExit := &pm.ExitHook{
		Action: func(state bool) {
			if !state {
				log.Errorf("dnsmasq for %s exited with an error", bridge.Name)
			}
		},
	}

	log.Debugf("dnsmasq(%s): %s", bridge.Name, args)
	_, err = pm.GetManager().RunCmd(cmd, onExit)
	return err
}

func bridgeNetworking(bridge *netlink.Bridge, network *BridgeNetwork) error {
	switch network.Mode {
	case StaticBridgeNetworkMode:
		return bridgeStaticNetworking(bridge, network)
	case DnsMasqBridgeNetworkMode:
		return bridgeDnsMasqNetworking(bridge, network)
	case NoneBridgeNetworkMode:
		//nothing to do
	default:
		return fmt.Errorf("invalid networking mode %s", network.Mode)
	}

	return nil
}

func bridgeCreate(cmd *core.Command) (interface{}, error) {
	var args BridgeCreateArguments
	if err := json.Unmarshal(*cmd.Arguments, &args); err != nil {
		return nil, err
	}
	var hw net.HardwareAddr

	if args.HwAddress != "" {
		var err error
		hw, err = net.ParseMAC(args.HwAddress)
		if err != nil {
			return nil, err
		}
	}

	bridge := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name:         args.Name,
			HardwareAddr: hw,
			TxQLen:       1000, //needed other wise bridge won't work
		},
	}

	if err := netlink.LinkAdd(bridge); err != nil {
		return nil, err
	}

	if err := netlink.LinkSetUp(bridge); err != nil {
		return nil, err
	}

	if err := bridgeNetworking(bridge, &args.Network); err != nil {
		//delete bridge?
		netlink.LinkDel(bridge)
		return nil, err
	}

	return nil, nil
}

func bridgeList(cmd *core.Command) (interface{}, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}

	bridges := make([]string, 0)
	for _, link := range links {
		if link.Type() == "bridge" {
			bridges = append(bridges, link.Attrs().Name)
		}
	}

	return bridges, nil
}

func bridgeDelete(cmd *core.Command) (interface{}, error) {
	var args BridgeDeleteArguments
	if err := json.Unmarshal(*cmd.Arguments, &args); err != nil {
		return nil, err
	}

	link, err := netlink.LinkByName(args.Name)
	if err != nil {
		return nil, err
	}

	if link.Type() != "bridge" {
		return nil, fmt.Errorf("bridge not found")
	}

	//make sure to stop dnsmasq, just in case it's running
	pm.GetManager().Kill(fmt.Sprintf("dnsmasq-%s", link.Attrs().Name))

	if err := netlink.LinkDel(link); err != nil {
		return nil, err
	}

	return nil, nil
}
