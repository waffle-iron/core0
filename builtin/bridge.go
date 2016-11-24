package builtin

import (
	"encoding/json"
	"fmt"
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
	"github.com/vishvananda/netlink"
	"net"
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

func bridgeNetworking(bridge *netlink.Bridge, network *BridgeNetwork) error {
	switch network.Mode {
	case StaticBridgeNetworkMode:
		return bridgeStaticNetworking(bridge, network)
	case DnsMasqBridgeNetworkMode:
	case NoneBridgeNetworkMode:
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

	if err := netlink.LinkDel(link); err != nil {
		return nil, err
	}

	return nil, nil
}
