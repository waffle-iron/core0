package agent

import (
	"fmt"
	"github.com/g8os/core/agent/lib/network"
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/settings"
	"log"
	"math/rand"
	"net"
	"net/http"
	"time"
)

const (
	FallbackControllerURL = "http://10.254.254.254:9066/"
)

var (
	FallbackMask   = net.IPMask([]byte{255, 255, 255, 0})
	FallbakIPRange = net.IP([]byte{10, 254, 254, 0})
)

type Bootstrap struct {
	i *settings.IncludedSettings
	t settings.StartupTree
}

func NewBootstrap() *Bootstrap {
	included, errors := settings.Settings.GetIncludedSettings()
	if len(errors) > 0 {
		for _, err := range errors {
			log.Println("ERROR: ", err)
		}
	}

	//startup services from [init, net[
	t, errors := included.GetStartupTree()

	if len(errors) > 0 {
		//print service tree errors (cyclic dependencies, or missing dependencies)
		for _, err := range errors {
			log.Println("ERROR: ", err)
		}
	}

	b := &Bootstrap{
		i: included,
		t: t,
	}

	return b
}

//TODO: POC bootstrap. This will most probably get rewritten when the process is clearer

func (b *Bootstrap) registerExtensions(extensions map[string]settings.Extension) {
	for extKey, extCfg := range extensions {
		var env []string
		if len(extCfg.Env) > 0 {
			env = make([]string, 0, len(extCfg.Env))
			for ek, ev := range extCfg.Env {
				env = append(env, fmt.Sprintf("%v=%v", ek, ev))
			}
		}

		pm.RegisterCmd(extKey, extCfg.Binary, extCfg.Cwd, extCfg.Args, env)
	}
}

func (b *Bootstrap) startupServices(s, e settings.After) {
	log.Printf("Starting up '%s' services\n", s)
	slice := b.t.Slice(s.Weight(), e.Weight())
	pm.GetManager().RunSlice(slice)
}

func (b *Bootstrap) pingController(controller *settings.Controller) bool {
	c := controller.GetClient()
	u := c.BuildURL("ping")

	log.Println("Pinging controller '%s'", u)
	r, err := c.Client.Get(u)
	if err != nil {
		log.Printf("ERROR: can't reach %s\n", controller.URL)
		return false
	}
	defer r.Body.Close()
	if r.StatusCode == http.StatusOK {
		return true
	}
	return false
}

func (b *Bootstrap) pingControllers() bool {
	for _, controller := range settings.Settings.Controllers {
		if ok := b.pingController(&controller); ok {
			//we were able to reach at least one controller
			return ok
		}
	}

	return false
}

func (b *Bootstrap) setupFallbackNetworking(interfaces []network.Interface) error {
	//we force static IPS on our network interfaces according to the following roles.
	for _, inf := range interfaces {
		inf.Clear()
		proto, _ := network.GetProtocol(network.ProtocolStatic)
		static := proto.(network.StaticProtocol)

		sip := net.IPv4(FallbakIPRange[0], FallbakIPRange[1], FallbakIPRange[2], 0)
		for sip[3] == 0 || sip[3] >= 254 {
			rand.Read(sip[3:])
			//TODO: check conflict
		}

		ip := &net.IPNet{
			IP:   sip,
			Mask: FallbackMask,
		}

		if err := static.ConfigureStatic(ip, inf.Name()); err != nil {
			log.Printf("ERROR: force static IP '%s' on '%s': %s\n", sip, inf.Name(), err)
		}

		if ok := b.pingControllers(); ok {
			return nil
		}

		inf.Clear()
		//reset interface to original setup.
		if err := inf.Configure(); err != nil {
			log.Println("ERROR:", err)
		}
	}

	return fmt.Errorf("no cotroller reachable with fallback plan")
}

func (b *Bootstrap) setupNetworking() error {
	netMgr, err := network.GetNetworkManager(settings.Settings.Main.Network)
	if err != nil {
		return err
	}

	interfaces, err := netMgr.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get network interfaces: %s", err)
	}

	//apply the interfaces settings as configured.
	for _, inf := range interfaces {
		log.Printf("Setting up interface '%s'\n", inf.Name())
		inf.Clear()
		if err := inf.Configure(); err != nil {
			log.Println("ERROR:", err)
		}
	}

	if ok := b.pingControllers(); ok {
		//we were able to reach one of the controllers.
		return nil
	}

	//force dhcp on all interfaces, and try again.
	dhcp, _ := network.GetProtocol(network.ProtocolDHCP)
	for _, inf := range interfaces {
		//try interfaces one by one
		if inf.Protocol() == network.ProtocolDHCP {
			//this interface already uses dhcp, no need to try that again
			continue
		}

		inf.Clear()
		if err := dhcp.Configure(netMgr, inf.Name()); err != nil {
			log.Println("ERROR: Force dhcp ", err)
		}

		if ok := b.pingControllers(); ok {
			return nil
		}

		inf.Clear()
		//reset interface to original setup.
		if err := inf.Configure(); err != nil {
			log.Println("ERROR:", err)
		}
	}

	//damn, we still can't reach the configured controller. we have to start our fallback plan
	return b.setupFallbackNetworking(interfaces)
}

//Bootstrap registers extensions and startup system services.
func (b *Bootstrap) Bootstrap() {
	//register core extensions
	b.registerExtensions(settings.Settings.Extension)

	//register included extensions
	b.registerExtensions(b.i.Extension)

	//start up all init services ([init, net[ slice)
	b.startupServices(settings.AfterInit, settings.AfterNet)

	for {
		err := b.setupNetworking()
		if err == nil {
			break
		}

		log.Printf("Failed to configure networking: %s\n", err)
		log.Println("Retrying in 2 seconds")

		time.Sleep(2 * time.Second)
		log.Println("Retrying setting up network")

		//DEBUG
		break
	}

	//start up all net services ([net, boot[ slice)
	b.startupServices(settings.AfterNet, settings.AfterBoot)

	//start up all boot services ([boot, end] slice)
	b.startupServices(settings.AfterBoot, settings.ToTheEnd)
}
