package agent

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

//main agent settings
type Settings struct {
	Main struct {
		Gid           int
		Nid           int
		MaxJobs       int
		MessageIdFile string
		HistoryFile   string
		Roles         []string
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
