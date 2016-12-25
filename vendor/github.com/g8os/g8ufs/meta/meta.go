package meta

import (
	"fmt"
	"github.com/op/go-logging"
)

var (
	log         = logging.MustGetLogger("meta")
	ErrNotFound = fmt.Errorf("not found")
)

type MetaData struct {
	Hash        string // file hash
	Size        uint64 // file size in bytes
	Uname       string // username (used for permissions)
	Uid         uint32
	Gname       string // groupname (used for permissions)
	Gid         uint32
	IsDir       bool
	Permissions uint32 // perissions (octal style)
	Ctime       uint64 // creation time
	Mtime       uint64 // modification time
	UserKey     string
	StoreKey    string
	Inode       uint64
}

type Meta interface {
	fmt.Stringer
	//base name
	Name() string
	Hash() string
	Stat() MetaData
	Children() <-chan Meta
}

type MetaStore interface {
	Populate(entry Entry) error
	Get(name string) (Meta, bool)
}
