package utils

import (
	"github.com/op/go-logging"
	"os"
)

var (
	log = logging.MustGetLogger("utils")
)

func In(l []string, x string) bool {
	for _, y := range l {
		if x == y {
			return true
		}
	}

	return false
}

func Exists(name string) bool {
	_, err := os.Stat(name)
	if err == nil {
		return true
	}

	return !os.IsNotExist(err)
}
