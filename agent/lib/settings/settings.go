package settings

import (
	"fmt"
	"github.com/g8os/core/agent/lib/network"
	"github.com/g8os/core/agent/lib/utils"
	"net/url"
	"strings"
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

	//Log address (for loggers that needs it)
	Address string
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

	key string
}

func (e *Extension) Key() string {
	return e.key
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

//Settings main agent settings
type AppSettings struct {
	Main struct {
		MaxJobs       int
		MessageIDFile string
		Include       string
		Network       string
	}

	Controllers map[string]Controller

	Extension map[string]Extension

	Logging map[string]Logger

	Stats struct {
		Interval int
		Ac       struct {
			Enabled     bool
			Controllers []string
		}
		Redis struct {
			Enabled       bool
			FlushInterval int
			Address       string
		}
	}

	Channel struct {
		Cmds []string
	}

	Hubble struct {
		Controllers []string
	}
}

var Settings AppSettings

func (s *AppSettings) Validate() []error {
	errors := make([]error, 0)
	for name, con := range s.Controllers {
		if u, err := url.Parse(con.URL); err != nil {
			verr := fmt.Errorf("[controller.%s] `url`: %s", name, err)
			errors = append(errors, verr)
		} else if !utils.InString([]string{"http", "https"}, strings.ToLower(u.Scheme)) {
			verr := fmt.Errorf("[controller.%s] `url` has unknown schema (%s)", name, u.Scheme)
			errors = append(errors, verr)
		}
	}

	return errors
}

func (s *AppSettings) GetNetworkManager() (network.NetworkManager, error) {
	return network.GetNetworkManager(s.Main.Network)
}

//GetSettings loads main settings from a filename
func LoadSettings(filename string) error {
	//that's the main config file, panic if can't load
	if err := utils.LoadTomlFile(filename, &Settings); err != nil {
		return err
	}

	return nil
}
