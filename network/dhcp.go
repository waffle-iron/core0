package network

import (
	"fmt"
	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
)

const (
	ProtocolDHCP = "dhcp"
)

func init() {
	protocols[ProtocolDHCP] = &dhcpProtocol{}
}

type DHCPProtocol interface {
	Protocol
	Stop(inf string)
}

type dhcpProtocol struct {
}

func (d *dhcpProtocol) Stop(inf string) {
	cmd := &core.Command{
		Name: "execute",
		Args: core.NewMapArgs(
			map[string]interface{}{
				"name": "dhcpcd",
				"args": []string{"-x", inf},
			},
		),
	}

	runner := pm.GetManager().RunCmd(cmd, false)

	runner.Wait()
}

func (d *dhcpProtocol) Configure(mgr NetworkManager, inf string) error {
	d.Stop(inf)

	cmd := &core.Command{
		Name: "execute",
		Args: core.NewMapArgs(
			map[string]interface{}{
				"name": "dhcpcd",
				"args": []string{"-w", inf},
			},
		),
	}

	runner := pm.GetManager().RunCmd(cmd, false)

	result := runner.Wait()

	if result == nil || result.State != core.StateSuccess {
		return fmt.Errorf("dhcpcd failed on interface %s: %s", inf, result.Streams)
	}

	return nil
}
