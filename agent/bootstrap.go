package agent

import (
	"fmt"
	"github.com/g8os/core/agent/lib/pm"
	"github.com/g8os/core/agent/lib/settings"
	"log"
	"time"
)

type Bootstrap struct {
	m *pm.PM
	i *settings.IncludedSettings
	t settings.StartupTree
}

func NewBootstrap(mgr *pm.PM) *Bootstrap {
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
		m: mgr,
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
	b.m.RunSlice(slice)
}

func (b *Bootstrap) startupNet() error {
	netmgr, err := settings.Settings.GetNetworkManager()
	if err != nil {
		return err
	}
	for _, inf := range netmgr.Interfaces() {
		log.Printf("Setting up interface '%s'\n", inf.Name())

		if err := inf.Configure(); err != nil {
			log.Println("ERROR:", err)
		}
	}

	return nil
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
		err := b.startupNet()
		if err == nil {
			break
		}

		log.Printf("Failed to configure networking: %s\n", err)
		log.Println("Retrying in 2 seconds")

		time.Sleep(2 * time.Second)
		log.Println("Retrying setting up network")
	}

	//start up all net services ([net, boot[ slice)
	b.startupServices(settings.AfterNet, settings.AfterBoot)

	//start up all boot services ([boot, end] slice)
	b.startupServices(settings.AfterBoot, settings.ToTheEnd)
}
