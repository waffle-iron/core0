package rofs

import (
	"github.com/g8os/g8ufs/meta"
	"github.com/g8os/g8ufs/storage"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/op/go-logging"
	"syscall"
)

var (
	log = logging.MustGetLogger("rofs")
)

type filesystem struct {
	pathfs.FileSystem
	cache   string
	store   meta.MetaStore
	storage storage.Storage
}

func New(storage storage.Storage, store meta.MetaStore, cache string) pathfs.FileSystem {
	fs := &filesystem{
		FileSystem: pathfs.NewDefaultFileSystem(),
		storage:    storage,
		store:      store,
		cache:      cache,
	}

	return pathfs.NewReadonlyFileSystem(fs)
}

func (fs *filesystem) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	log.Debugf("GetAttr %s", name)
	m, ok := fs.store.Get(name)
	if !ok {

		return nil, fuse.ENOENT
	}

	stat := m.Stat()

	mode := syscall.S_IFREG

	if stat.IsDir {
		mode = syscall.S_IFDIR
	}

	return &fuse.Attr{
		Ino:   stat.Inode,
		Size:  stat.Size,
		Mtime: stat.Mtime,
		Mode:  uint32(mode) | stat.Permissions,
	}, fuse.OK
}

func (fs *filesystem) Open(name string, flags uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	log.Debugf("Open %s", name)
	if flags&fuse.O_ANYWRITE != 0 {
		return nil, fuse.EPERM
	}
	m, ok := fs.store.Get(name)
	if !ok {
		return nil, fuse.ENOENT
	}
	f, err := fs.checkAndGet(m)
	if err != nil {
		log.Errorf("Failed to open/download the file: %s", err)
	}

	return nodefs.NewReadOnlyFile(nodefs.NewLoopbackFile(f)), fuse.OK
}

func (fs *filesystem) OpenDir(name string, context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	log.Debugf("OpenDir %s", name)
	m, ok := fs.store.Get(name)
	if !ok {
		return nil, fuse.ENOENT
	}
	var entries []fuse.DirEntry
	for child := range m.Children() {
		stat := child.Stat()
		mode := syscall.S_IFREG
		if stat.IsDir {
			mode = syscall.S_IFDIR
		}

		entries = append(entries, fuse.DirEntry{
			Mode: uint32(mode),
			Name: child.Name(),
		})
	}

	return entries, fuse.OK
}

func (fs *filesystem) String() string {
	return "G8UFS"
}

func (fs *filesystem) Access(name string, mode uint32, context *fuse.Context) fuse.Status {
	return fuse.OK
}

func (fs *filesystem) GetXAttr(name string, attr string, context *fuse.Context) ([]byte, fuse.Status) {
	return nil, fuse.ENOSYS
}

func (fs *filesystem) ListXAttr(name string, context *fuse.Context) ([]string, fuse.Status) {
	return nil, fuse.ENOSYS
}

func (fs *filesystem) StatFs(name string) *fuse.StatfsOut {
	return &fuse.StatfsOut{}
}
