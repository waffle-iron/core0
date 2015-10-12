package settings

import (
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"io"
	"io/ioutil"
	"path"
	"sort"
)

const (
	CONFIG_SUFFIX = ".toml"
)

//logger settings
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

//external cmd config
type Extension struct {
	//binary to execute
	Binary string
	//script search path
	Cwd string
	//(optional) Env variables
	Env map[string]string

	Args []string
}

type Security struct {
	CertificateAuthority string
	ClientCertificate    string
	ClientCertificateKey string
}

type Controller struct {
	URL      string
	Security Security
}

type StartupCmd struct {
	Name string
	Data string
	Args map[string]interface{}
}

func (up StartupCmd) Hash() string {
	h := md5.New()

	io.WriteString(h, fmt.Sprintf("Name:%s,", up.Name))
	io.WriteString(h, fmt.Sprintf("Data:%s,", up.Data))

	keys := make([]string, 0, len(up.Args))
	for key, _ := range up.Args {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	for _, key := range keys {
		io.WriteString(h, fmt.Sprintf("%s:%v,", key, up.Args[key]))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

//main agent settings
type Settings struct {
	Main struct {
		Gid           int
		Nid           int
		MaxJobs       int
		MessageIdFile string
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

//partial loadable settings
type PartialSettings struct {
	Extensions map[string]Extension

	Startup map[string]StartupCmd
}

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
		if len(name) <= len(CONFIG_SUFFIX) {
			//file name too short to be a config file (shorter than the extension)
			continue
		}
		if name[len(name)-len(CONFIG_SUFFIX):] != CONFIG_SUFFIX {
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
				return nil, errors.New(fmt.Sprintf("Extension override in '%s' name '%s'", partialPath, key))
			}

			partial.Extensions[key] = ext
		}

		for key, startup := range partialCfg.Startup {
			_, m := settings.Startup[key]
			_, p := partial.Startup[key]
			if m || p {
				return nil, errors.New(fmt.Sprintf("Startup command override in '%s' name '%s'", partialPath, key))
			}

			partial.Startup[key] = startup
		}
	}

	return partial, nil
}

func GetSettings(filename string) *Settings {
	settings := &Settings{}

	//that's the main config file, panic if can't load
	if err := utils.LoadTomlFile(filename, settings); err != nil {
		panic(err)
	}

	return settings
}
