package bootstrap

import (
	"github.com/op/go-logging"
	"syscall"
)

var (
	log = logging.MustGetLogger("bootstrap")
)

type Bootstrap struct {
}

func NewBootstrap() *Bootstrap {
	return &Bootstrap{}
}

//Bootstrap registers extensions and startup system services.
func (b *Bootstrap) Bootstrap() error {
	log.Infof("Mounting proc")
	return syscall.Mount("none", "/proc", "proc", 0, "")
}
