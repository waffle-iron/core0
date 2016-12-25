package storage

import (
	"github.com/op/go-logging"
	"io"
)

var (
	log = logging.MustGetLogger("storage")
)

type Storage interface {
	Get(key string) (io.ReadCloser, error)
}
