package agent

import (
	"crypto/md5"
	"fmt"
	"io"
	"sort"
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
