package settings

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

type AppOptions struct {
	cfg   string
	gid   int
	nid   int
	roles string
}

func (o *AppOptions) Config() string {
	return o.cfg
}

func (o *AppOptions) Gid() int {
	return o.gid
}

func (o *AppOptions) Nid() int {
	return o.nid
}

func (o *AppOptions) Roles() []string {
	if o.roles == "" {
		return nil
	}
	return strings.Split(o.roles, ",")
}

func (o *AppOptions) Validate() []error {
	errors := make([]error, 0)
	if o.gid == 0 {
		errors = append(errors, fmt.Errorf("Gid can't be 0"))
	}

	if o.nid == 0 {
		errors = append(errors, fmt.Errorf("Nid can't be 0"))
	}

	return errors
}

var Options AppOptions

func init() {
	help := false
	flag.BoolVar(&help, "h", false, "Print this help screen")
	flag.StringVar(&Options.cfg, "c", "/etc/g8os/g8os.toml", "Path to config file")

	flag.IntVar(&Options.gid, "gid", 0, "Grid ID")
	flag.IntVar(&Options.nid, "nid", 0, "Node ID")
	flag.StringVar(&Options.roles, "roles", "", "A comma separated list of roles")
	flag.Parse()

	printHelp := func() {
		fmt.Println("core [options]")
		flag.PrintDefaults()
	}

	if help {
		printHelp()
		os.Exit(0)
	}
}
