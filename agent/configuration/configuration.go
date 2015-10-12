package configuration

import (
	"fmt"
	"log"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/settings"
	"github.com/Jumpscale/agent2/agent/lib/utils"

	"github.com/rjeczalik/notify"
)

//WatchAndApply watches and applies changes of configuration folder
func WatchAndApply(mgr *pm.PM, cfg *settings.Settings) {
	if cfg.Main.Include == "" {
		return
	}

	type PlaceHolder struct {
		Hash string
		Id   string
	}

	extensions := make([]string, 0, 100)
	commands := make(map[string]PlaceHolder)

	apply := func() error {
		partial, err := settings.GetPartialSettings(cfg)
		if err != nil {
			log.Println(err)
			return err
		}
		//first, unregister all the extensions from the PM.
		//that might have been registered before.
		for _, extKey := range extensions {
			pm.UnregisterCmd(extKey)
		}
		extensions = extensions[:]

		//register the execute commands
		for extKey, extCfg := range partial.Extensions {
			var env []string
			if len(extCfg.Env) > 0 {
				env = make([]string, 0, len(extCfg.Env))
				for ek, ev := range extCfg.Env {
					env = append(env, fmt.Sprintf("%v=%v", ek, ev))
				}
			}

			pm.RegisterCmd(extKey, extCfg.Binary, extCfg.Cwd, extCfg.Args, env)
			extensions = append(extensions, extKey)
		}

		//Simple diff to find out which command needs to be restarted
		//and witch needs to be stopped totally
		var running []string
		for name, startup := range partial.Startup {
			hash := startup.Hash()
			if ph, ok := commands[name]; ok {
				//name already tracked
				if ph.Hash == hash {
					//no changes to the command.
					running = append(running, name)
					continue
				} else {
					mgr.Kill(ph.Id)
					delete(commands, name)
				}
			}

			if startup.Args == nil {
				startup.Args = make(map[string]interface{})
			}

			id := uuid.New()

			cmd := &pm.Cmd{
				Gid:  cfg.Main.Gid,
				Nid:  cfg.Main.Nid,
				Id:   id,
				Name: startup.Name,
				Data: startup.Data,
				Args: pm.NewMapArgs(startup.Args),
			}

			meterInt := cmd.Args.GetInt("stats_interval")
			if meterInt == 0 {
				cmd.Args.Set("stats_interval", cfg.Stats.Interval)
			}

			mgr.RunCmd(cmd)
			commands[name] = PlaceHolder{
				Hash: hash,
				Id:   id,
			}
			running = append(running, name)
		}

		//kill commands that are removed from config
		for name, ph := range commands {
			if !utils.InString(running, name) {
				mgr.Kill(ph.Id)
				delete(commands, name)
			}
		}

		return nil
	}

	watch := func(folder string) error {

		// Make the channel buffered to ensure no event is dropped. Notify will drop
		// an event if the receiver is not able to keep up the sending pace.
		fsevents := make(chan notify.EventInfo, 4)

		// Set up a watchpoint listening on events within current working directory.
		// Dispatch each create and remove events separately to c.
		if err := notify.Watch(folder, fsevents, notify.Write, notify.Create, notify.Remove); err != nil {
			log.Fatal(err)
		}
		defer notify.Stop(fsevents)

		for {
			// Block until an event is received.
			fsevent := <-fsevents
			path := fsevent.Path()

			if !strings.HasSuffix(path, settings.CONFIG_SUFFIX) {
				//file name too short to be a config file (shorter than the extension)
				continue
			}
			log.Println("Configuration file changed:", fsevent)
			apply()
		}

	}

	apply()
	go watch(cfg.Main.Include)
}
