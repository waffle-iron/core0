package network

import (
	"fmt"
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/pm/core"
)

const (
	ProtocolDHCP = "dhcp"
)

func init() {
	protocols[ProtocolDHCP] = &dhcpProtocol{}
}

type dhcpProtocol struct {
}

func (d *dhcpProtocol) killDhcpcd(inf string) {
	cmd := &core.Cmd{
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
	d.killDhcpcd(inf)

	cmd := &core.Cmd{
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
