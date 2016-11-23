package files

import (
	"time"

	"github.com/g8os/fs/config"

	"github.com/g8os/fs/meta"
	"github.com/g8os/fs/storage"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/op/go-logging"
)

var (
	log = logging.MustGetLogger("files")
)

// FS defines g8os fuse filesystem
type FS struct {
	mountpoint string
	backend    *config.Backend
	stor       storage.Storage
	conn       *nodefs.FileSystemConnector
	pathFs     *pathfs.PathNodeFs
	server     *fuse.Server
	meta       meta.MetaStore
}

// NewFS creates new fuse filesystem using hanwen/go-fuse lib
func NewFS(mountpoint string, backend *config.Backend, storage storage.Storage, meta meta.MetaStore, readOnly bool) (*FS, error) {
	fs := &FS{
		mountpoint: mountpoint,
		backend:    backend,
		stor:       storage,
		meta:       meta,
	}

	filesys := newFileSystem(fs)
	if readOnly {
		filesys = pathfs.NewReadonlyFileSystem(filesys)
	}

	opts := &nodefs.Options{
		// These options are to be compatible with libfuse defaults,
		// making benchmarking easier.
		NegativeTimeout: time.Second,
		AttrTimeout:     time.Second,
		EntryTimeout:    time.Second,
	}
	fs.pathFs = pathfs.NewPathNodeFs(filesys, nil)
	fs.conn = nodefs.NewFileSystemConnector(fs.pathFs.Root(), opts)

	mOpts := &fuse.MountOptions{
		AllowOther: true,
		Name:       "g8osfs",
		FsName:     fs.backend.Path,
	}
	state, err := fuse.NewServer(fs.conn.RawFS(), mountpoint, mOpts)
	if err != nil {
		return fs, err
	}
	fs.server = state
	// fs.server.SetDebug(true)

	return fs, nil
}

func (fs *FS) Serve() {
	fs.server.Serve()
}

func (fs *FS) Unmount() error {
	return fs.server.Unmount()
}

func (fs *FS) WaitMount() {
	fs.server.WaitMount()
}
