package options

import (
	"flag"
	"fmt"
	"os"
)

type AppOptions struct {
	cfg   string
	roles string
}

func (o *AppOptions) Config() string {
	return o.cfg
}

var Options AppOptions

func init() {
	help := false
	flag.BoolVar(&help, "h", false, "Print this help screen")
	flag.StringVar(&Options.cfg, "c", "/etc/g8os/g8os.toml", "Path to config file")
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
