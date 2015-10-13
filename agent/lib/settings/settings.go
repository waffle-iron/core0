package settings

import (
	"crypto/md5"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"io"
	"io/ioutil"
	"path"
	"sort"
)

const (
	//ConfigSuffix config file ext
	ConfigSuffix = ".toml"
)

//Logger settings
type Logger struct {
	//logger type, now only 'db' and 'ac' are supported
	Type string
	//list of controlles base URLs
	Controllers []string
	//Process which levels
	Levels []int

	//Log dir (for loggers that needs it)
	LogDir string
	//Flush interval (for loggers that needs it)
	FlushInt int
	//Flush batch size (for loggers that needs it)
	BatchSize int
}

//Extension cmd config
type Extension struct {
	//binary to execute
	Binary string
	//script search path
	Cwd string
	//(optional) Env variables
	Env map[string]string

	Args []string
}

//Security certificate path
type Security struct {
	CertificateAuthority string
	ClientCertificate    string
	ClientCertificateKey string
}

//Controller url and certificates
type Controller struct {
	URL      string
	Security Security
}

//StartupCmd startup command config
type StartupCmd struct {
	Name string
	Data string
	Args map[string]interface{}
}

//Hash calculates a hash for the startup command, identical commands should have identical hashes.
func (up StartupCmd) Hash() string {
	h := md5.New()

	io.WriteString(h, fmt.Sprintf("Name:%s,", up.Name))
	io.WriteString(h, fmt.Sprintf("Data:%s,", up.Data))

	keys := make([]string, 0, len(up.Args))
	for key := range up.Args {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	for _, key := range keys {
		io.WriteString(h, fmt.Sprintf("%s:%v,", key, up.Args[key]))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

//Settings main agent settings
type Settings struct {
	Main struct {
		Gid           int
		Nid           int
		MaxJobs       int
		MessageIDFile string
		HistoryFile   string
		Roles         []string
		Include       string
	}

	Controllers map[string]Controller

	Extensions map[string]Extension

	Logging map[string]Logger

	Stats struct {
		Interval    int
		Controllers []string
	}

	Channel struct {
		Cmds []string
	}

	Hubble struct {
		Controllers []string
	}

	Startup map[string]StartupCmd
}

//PartialSettings loadable settings
type PartialSettings struct {
	Extensions map[string]Extension

	Startup map[string]StartupCmd
}

//GetPartialSettings loads partial settings according to main configurations
func GetPartialSettings(settings *Settings) (*PartialSettings, error) {
	partial := &PartialSettings{
		Extensions: make(map[string]Extension),
		Startup:    make(map[string]StartupCmd),
	}

	if settings.Main.Include == "" {
		return partial, nil
	}

	infos, err := ioutil.ReadDir(settings.Main.Include)
	if err != nil {
		return nil, err
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

		partialCfg := PartialSettings{}
		partialPath := path.Join(settings.Main.Include, name)

		err := utils.LoadTomlFile(partialPath, &partialCfg)
		if err != nil {
			return nil, err
		}

		//merge into settings
		for key, ext := range partialCfg.Extensions {
			_, m := settings.Extensions[key]
			_, p := partial.Extensions[key]
			if m || p {
				return nil, fmt.Errorf("Extension override in '%s' name '%s'", partialPath, key)
			}

			partial.Extensions[key] = ext
		}

		for key, startup := range partialCfg.Startup {
			_, m := settings.Startup[key]
			_, p := partial.Startup[key]
			if m || p {
				return nil, fmt.Errorf("Startup command override in '%s' name '%s'", partialPath, key)
			}

			partial.Startup[key] = startup
		}
	}

	return partial, nil
}

//GetSettings loads main settings from a filename
func GetSettings(filename string) *Settings {
	settings := &Settings{}

	//that's the main config file, panic if can't load
	if err := utils.LoadTomlFile(filename, settings); err != nil {
		panic(err)
	}

	return settings
}
