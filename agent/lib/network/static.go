package network

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"log"
	"net"
)

var defaultRouteIp *net.IPNet

func init() {
	defaultRouteIp, _ = netlink.ParseIPNet("0.0.0.0/0")

	protocols["static"] = &staticProtocol{}
}

type staticProtocol struct{}

func (s *staticProtocol) Configure(n *networkingSettings, inf string) error {
	link, err := netlink.LinkByName(inf)
	if err != nil {
		return err
	}

	setting, ok := n.Static[inf]
	if !ok {
		return fmt.Errorf("missing static configuration for interface '%s'", inf)
	}

	ip, err := netlink.ParseIPNet(setting.IP)

	addr := &netlink.Addr{IPNet: ip}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	log.Println("Addresses: ", addrs)
	if err != nil {
		return err
	}

	exists := false
	for _, a := range addrs {
		if a.IPNet.String() == addr.IPNet.String() {
			exists = true
			break
		}
	}

	if !exists {
		if err := netlink.AddrAdd(link, addr); err != nil {
			return err
		}
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return err
	}

	if setting.Gateway == "" {
		return nil
	}

	//setting up gateway.
	rip := net.ParseIP(setting.Gateway)
	if rip == nil {
		return fmt.Errorf("invalid ip for gateway '%s'", rip)
	}

	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Src:       addr.IPNet.IP,
		Gw:        rip,
	}

	return netlink.RouteAdd(route)
}
