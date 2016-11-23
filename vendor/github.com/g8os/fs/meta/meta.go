package meta

import (
	"fmt"
	"github.com/op/go-logging"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
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
	Permissions uint32 // perissions (octal style)
	Filetype    uint32 // golang depend os.FileMode id
	Ctime       uint64 // creation time
	Mtime       uint64 // modification time
	Extended    string // extended attribute (see python flist doc)
	DevMajor    int64  // block/char device major id
	DevMinor    int64  // block/char device minor id
	UserKey     string
	StoreKey    string
	Inode       uint64
}

type MetaState uint32

const (
	MetaStateMask MetaState = 0500
	MetaInitial   MetaState = 0400
	MetaModified  MetaState = 0200
	MetaDeleted   MetaState = 0100
)

func (s MetaState) Modified() bool {
	return s&MetaModified != 0
}

//
//func (s MetaState) Deleted() bool {
//	return s&MetaDeleted != 0
//}

func (s MetaState) SetModified(m bool) MetaState {
	if m {
		return MetaState(s | MetaModified)
	} else {
		return MetaState(s & (^MetaModified))
	}
}

//
//func (s MetaState) SetDeleted(m bool) MetaState {
//	if m {
//		return MetaState(s | MetaDeleted)
//	} else {
//		return MetaState(s & (^MetaDeleted))
//	}
//}

type Meta interface {
	fmt.Stringer
	//base name
	Name() string
	Stat() MetaState
	SetStat(state MetaState)
	Load() (*MetaData, error)
	Save(meta *MetaData) error
	Children() <-chan Meta
}

type MetaStore interface {
	Populate(plist string, trim string) error
	Get(name string) (Meta, bool)
	CreateFile(name string) (Meta, error)
	CreateDir(name string) (Meta, error)
	Delete(meta Meta) error
}

/*
type Entry struct {
	Path string
	Hash string
	Size int64
}
*/

type Entry struct {
	Filepath    string    // complete filepath
	Hash        string    // file hash
	Filesize    int64     // file size in bytes
	Uname       string    // username (used for permissions)
	Gname       string    // groupname (used for permissions)
	Permissions int64     // perissions (octal style)
	Filetype    uint32    // golang depend os.FileMode id
	Ctime       time.Time // creation time
	Mtime       time.Time // modification time
	Extended    string    // extended attribute (see python flist doc)
	DevMajor    int64     // block/char device major id
	DevMinor    int64     // block/char device minor id
}

func ParseLine(line string, trim string) (*Entry, error) {
	if line == "" {
		err := fmt.Errorf("Cannot parse empty lines\n")
		return nil, err
	}

	// split line
	items := strings.Split(line, "|")

	if len(items) < 10 {
		err := fmt.Errorf("Flist item: malformed line, at least 10 fields expected, %d found\n", len(items))
		return nil, err
	}

	//
	// file stats
	//
	filepath := strings.TrimPrefix(items[0], trim)

	length, err := strconv.ParseInt(items[2], 10, 64)
	if err != nil {
		fmt.Errorf("Error parsing filesize: %v\n", err)
		return nil, err
	}

	perms, err := strconv.ParseInt(items[5], 8, 64)
	if err != nil {
		fmt.Errorf("Error parsing permissions: %v\n", err)
		return nil, err
	}

	//
	// file type
	//
	ftype, err := strconv.Atoi(items[6])
	fileType := os.ModeDir
	if err != nil {
		fmt.Errorf("Error parsing filetype: %v\n", err)
		return nil, err
	}

	devMajor := int64(0)
	devMinor := int64(0)

	if ftype == 3 || ftype == 5 {
		temp := strings.Split(items[9], ",")

		devMajor, err = strconv.ParseInt(temp[0], 10, 64)
		if err != nil {
			fmt.Errorf("Error parsing device major id: %v\n", err)
			return nil, err
		}

		devMinor, err = strconv.ParseInt(temp[1], 10, 64)
		if err != nil {
			fmt.Errorf("Error parsing device minor id: %v\n", err)
			return nil, err
		}
	}

	if ftype == 0 {
		fileType = syscall.S_IFSOCK
	}
	if ftype == 1 {
		fileType = syscall.S_IFLNK
	}
	if ftype == 2 {
		fileType = syscall.S_IFREG
	}
	if ftype == 3 {
		fileType = syscall.S_IFBLK
	}
	if ftype == 4 {
		fileType = syscall.S_IFDIR
	}
	if ftype == 5 {
		fileType = syscall.S_IFCHR
	}
	if ftype == 6 {
		fileType = syscall.S_IFIFO
	}

	//
	// file times
	//
	ctime, err := strconv.ParseInt(items[7], 10, 64)
	if err != nil {
		fmt.Errorf("Error parsing creation time: %v\n", err)
		return nil, err
	}

	mtime, err := strconv.ParseInt(items[8], 10, 64)
	if err != nil {
		fmt.Errorf("Error parsing modification time: %v\n", err)
		return nil, err
	}

	return &Entry{
		Filepath:    filepath,
		Hash:        items[1],
		Filesize:    length,
		Uname:       items[3],
		Gname:       items[4],
		Permissions: perms,
		Filetype:    uint32(fileType),
		Ctime:       time.Unix(ctime, 0),
		Mtime:       time.Unix(mtime, 0),
		Extended:    items[9],
		DevMajor:    devMajor,
		DevMinor:    devMinor,
	}, nil
}
