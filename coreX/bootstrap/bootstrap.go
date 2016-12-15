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
	if err := syscall.Mount("none", "/proc", "proc", 0, ""); err != nil {
		return err
	}

	if err := syscall.Mount("none", "/dev", "devtmpfs", 0, ""); err != nil {
		return err
	}

	if err := syscall.Mount("none", "/dev/pts", "devpts", 0, ""); err != nil {
		return err
	}

	return nil
}
