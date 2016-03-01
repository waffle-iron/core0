package settings

import (
	"fmt"
	"github.com/g8os/core/agent/lib/utils"
	"io/ioutil"
	"log"
	"path"
)

type After string

const (
	//Init happens before handshake
	AfterInit = After("init")

	//Core happens with core is up and running (also networking)
	AfterNet = After("net")

	//Default for startup commands that doesn't specify dependency
	AfterBoot = After("boot")
)

var (
	CyclicDependency = fmt.Errorf("cyclic dependency")

	Priority = map[After]int64{
		AfterInit: 1,
		AfterNet:  100,
		AfterBoot: 1000,
	}
)

type IncludedSettings struct {
	Extensions map[string]Extension
	Startups   map[string]Startup
}

//GetPartialSettings loads partial settings according to main configurations
func (s *AppSettings) GetIncludedSettings() (partial *IncludedSettings) {
	partial = &IncludedSettings{
		Extensions: make(map[string]Extension),
		Startups:   make(map[string]Startup),
	}

	if s.Main.Include == "" {
		return partial
	}

	infos, err := ioutil.ReadDir(s.Main.Include)
	if err != nil {
		log.Println("Failed to read dir ", s.Main.Include, err)
		return
	}

	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		name := info.Name()
		if len(name) <= len(ConfigSuffix) {
			//file name too short to be a config file (shorter than the extension)
			continue
		}
		if name[len(name)-len(ConfigSuffix):] != ConfigSuffix {
			continue
		}

		partialCfg := IncludedSettings{}
		partialPath := path.Join(s.Main.Include, name)

		err := utils.LoadTomlFile(partialPath, &partialCfg)
		if err != nil {
			log.Println("ERROR: failed to load file ", partialPath, err)
			continue
		}

		//merge into settings
		for key, ext := range partialCfg.Extensions {
			_, m := s.Extensions[key]
			_, p := partial.Extensions[key]
			if m || p {
				log.Printf("Extension override in '%s' name '%s'\n", partialPath, key)
				continue
			}

			ext.key = key
			partial.Extensions[key] = ext
		}

		for key, startup := range partialCfg.Startups {
			startup.key = key
			partial.Startups[key] = startup
		}
	}

	return
}
