package meta

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
)

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
	DevMajor    int       // block/char device major id
	DevMinor    int       // block/char device minor id
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

	devMajor := 0
	devMinor := 0

	if ftype == 3 || ftype == 5 {
		temp := strings.Split(items[9], ",")

		dj, err := strconv.ParseInt(temp[0], 10, 0)
		if err != nil {
			fmt.Errorf("Error parsing device major id: %v\n", err)
			return nil, err
		}
		devMajor = int(dj)

		di, err := strconv.ParseInt(temp[1], 10, 0)
		if err != nil {
			fmt.Errorf("Error parsing device minor id: %v\n", err)
			return nil, err
		}
		devMinor = int(di)
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

func Populate(store MetaStore, plist string, writable string, trim string) error {
	f, err := os.Open(plist)
	if err != nil {
		return err
	}

	defer f.Close()
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		entry, err := ParseLine(line, trim)
		if err != nil {
			return err
		}

		if entry.Filetype == syscall.S_IFDIR ||
			entry.Filetype == syscall.S_IFREG {
			store.Populate(*entry)
			continue
		}

		name := path.Join(writable, entry.Filepath)
		dir := path.Dir(name)
		os.MkdirAll(dir, 0755)

		if entry.Filetype == syscall.S_IFLNK {
			if err := os.Symlink(entry.Extended, name); err != nil {
				log.Warningf("file '%s' exists", name)
			}
			continue
		}

		//mknode for all other types
		dev := entry.DevMajor<<8 | entry.DevMinor
		if err := syscall.Mknod(name, entry.Filetype|0644, dev); err != nil {
			log.Warningf("file '%s' exists", name)
		}
	}

	return nil
}
