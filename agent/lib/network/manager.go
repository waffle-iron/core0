package network

import (
	"fmt"
	"github.com/g8os/core/agent/lib/utils"
)

type interfaceSettings struct {
	Protocol string
}

type staticSettings struct {
	IP      string
	Gateway string
}

type networkingSettings struct {
	Interface map[string]interfaceSettings

	Static map[string]staticSettings
}

type Interface interface {
	Name() string
	Protocol() string
	Configure() error
}

type NetworkManager interface {
	Interfaces() []Interface
}

type networkInterface struct {
	name               string
	settings           interfaceSettings
	networkingSettings *networkingSettings
}

func (i *networkInterface) Name() string {
	return i.name
}

func (i *networkInterface) Protocol() string {
	return i.settings.Protocol
}

func (i *networkInterface) Configure() error {
	protocol, ok := protocols[i.Protocol()]
	if !ok {
		return fmt.Errorf("unkonwn networking protocol '%s'", i.Protocol())
	}

	return protocol.Configure(i.networkingSettings, i.Name())
}

type networkManager struct {
	settings *networkingSettings
}

//GetNetworkManager gets a new manager and loads settings from the provided toml file
func GetNetworkManager(filename string) (NetworkManager, error) {
	networking := &networkingSettings{}
	if err := utils.LoadTomlFile(filename, networking); err != nil {
		return nil, err
	}

	return &networkManager{networking}, nil
}

func (m *networkManager) Interfaces() []Interface {
	l := make([]Interface, 0)
	for k, inf := range m.settings.Interface {
		net := &networkInterface{
			name:               k,
			settings:           inf,
			networkingSettings: m.settings,
		}

		l = append(l, net)
	}

	return l
}
