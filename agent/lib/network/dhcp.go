package network

func init() {
	protocols["dhcp"] = &dhcpProtocol{}
}

type dhcpProtocol struct {
}

func (d *dhcpProtocol) Configure(n *networkingSettings, inf string) error {

	return nil
}
