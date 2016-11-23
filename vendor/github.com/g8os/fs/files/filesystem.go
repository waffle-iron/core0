package files

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dsnet/compress/brotli"
	"github.com/g8os/fs/crypto"
	"github.com/g8os/fs/meta"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"path"
)

// filesystem represents g8os filesystem
type fileSystem struct {
	// TODO - this should need default fill in.
	pathfs.FileSystem
	Root string
	*FS
}

// create filesystem object
func newFileSystem(fs *FS) pathfs.FileSystem {
	return &fileSystem{
		FileSystem: NewDefaultFileSystem(),
		Root:       fs.backend.Path,
		FS:         fs,
	}
}

func (fs *fileSystem) OnMount(nodeFs *pathfs.PathNodeFs) {
	log.Debug("OnMount")
}

func (fs *fileSystem) OnUnmount() {
	log.Debug("OnUnmount")
}

func (fs *fileSystem) GetPath(relPath string) string {
	return filepath.Join(fs.Root, relPath)
}

func (fs *fileSystem) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	var err error = nil
	attr := &fuse.Attr{}

	log.Debugf("GetAttr '%v'", name)

	m, exists := fs.meta.Get(name)

	if !exists {
		return nil, fuse.ENOENT
	}

	metadata, err := m.Load()
	if err != nil {
		return nil, fuse.ToStatus(err)
	}

	var st syscall.Stat_t
	err = syscall.Stat(fs.GetPath(name), &st)
	if err == nil {
		log.Debugf("GetAttr %v: metadata, forwarding from backend", fs.GetPath(name))
		attr.FromStat(&st)
		attr.Ino = metadata.Inode
		return attr, fuse.OK
	}

	attr.Size = metadata.Size
	attr.Mode = metadata.Filetype | metadata.Permissions

	if metadata.Filetype == syscall.S_IFLNK {
		attr.Mode = metadata.Filetype | 0777
		if err := syscall.Lstat(metadata.Extended, &st); err == nil {
			attr.Uid = st.Uid
			attr.Gid = st.Gid
			attr.Ctime = uint64(st.Ctim.Sec)
			attr.Mtime = uint64(st.Mtim.Sec)
		}
	} else {
		attr.Ctime = metadata.Ctime
		attr.Mtime = metadata.Mtime
		attr.Uid = metadata.Uid
		attr.Gid = metadata.Gid
	}

	attr.Ino = metadata.Inode

	// block and character devices
	if metadata.Filetype == syscall.S_IFCHR || metadata.Filetype == syscall.S_IFBLK {
		attr.Rdev = uint32((metadata.DevMajor * 256) + metadata.DevMinor)
	}

	return attr, fuse.OK
}

// Open opens a file.
// Download it from stor if file not exist
func (fs *fileSystem) Open(name string, flags uint32, context *fuse.Context) (fuseFile nodefs.File, status fuse.Status) {
	var st syscall.Stat_t

	log.Debugf("Open %v", name)

	m, exists := fs.meta.Get(name)
	if flags&uint32(os.O_RDONLY) > 0 && !exists {
		return nil, fuse.ENOENT
	}

	err := syscall.Lstat(fs.GetPath(name), &st)

	_, exists = fs.meta.Get(path.Dir(name))
	if !exists {
		return nil, fuse.ENOENT
	}

	dir := path.Dir(name)
	if _, ok := fs.meta.Get(dir); ok {
		os.MkdirAll(fs.GetPath(dir), 0755)
	} else {
		return nil, fuse.ENOENT
	}

	if exists && os.IsNotExist(err) {
		if err := fs.download(m, fs.GetPath(name)); err != nil {
			log.Errorf("Error getting file from stor: %s", err)
			return nil, fuse.EIO
		}

		return fs.Open(name, flags, context)
	}

	//we can reach here only if we are in create mode.
	//we need to create a meta file to associate with this file.
	m, err = fs.meta.CreateFile(name)
	if err != nil {
		return nil, fuse.ToStatus(err)
	}

	data, err := m.Load()
	if err != nil {
		return nil, fuse.ToStatus(err)
	}

	file, err := os.OpenFile(fs.GetPath(name), int(flags), 0)
	if err != nil {
		return nil, fuse.ToStatus(err)
	}

	if err := syscall.Stat(fs.GetPath(name), &st); err != nil {
		return nil, fuse.ToStatus(err)
	}

	m.Save(&meta.MetaData{
		Inode:       data.Inode,
		Size:        uint64(st.Size),
		Filetype:    syscall.S_IFREG,
		Uid:         st.Uid,
		Gid:         st.Gid,
		Permissions: st.Mode | uint32(os.ModePerm),
		Ctime:       uint64(st.Ctim.Sec),
		Mtime:       uint64(st.Mtim.Sec),
	})

	return NewLoopbackFile(m, file), fuse.OK
}

func (fs *fileSystem) Truncate(path string, offset uint64, context *fuse.Context) (code fuse.Status) {
	m, err := fs.meta.CreateFile(path)
	if err != nil {
		return fuse.ToStatus(err)
	}
	m.SetStat(m.Stat().SetModified(true))
	return fuse.ToStatus(os.Truncate(fs.GetPath(path), int64(offset)))
}

func (fs *fileSystem) Chmod(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	fullPath := fs.GetPath(name)
	log.Debugf("Chmod %v", fullPath)

	return fuse.ToStatus(os.Chmod(fullPath, os.FileMode(mode)))
}

func (fs *fileSystem) Chown(name string, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	fullPath := fs.GetPath(name)
	log.Debugf("Chown %v", fullPath)

	return fuse.ToStatus(os.Chown(fullPath, int(uid), int(gid)))
}

func (fs *fileSystem) Readlink(name string, context *fuse.Context) (out string, code fuse.Status) {
	var err error = nil
	log.Debugf("ReadLink %v", name)

	m, exists := fs.meta.Get(name)
	if !exists {
		return "", fuse.ENOENT
	}
	metadata, err := m.Load()
	if err != nil {
		return "", fuse.ToStatus(err)
	}

	if metadata.Filetype != syscall.S_IFLNK {
		return "", fuse.EIO
	}

	return metadata.Extended, fuse.OK
}

// Don't use os.Remove, it removes twice (unlink followed by rmdir).
func (fs *fileSystem) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	fullPath := fs.GetPath(name)
	m, exists := fs.meta.Get(name)
	if !exists {
		return fuse.ENOENT
	}

	if err := os.Remove(fullPath); err != nil {
		log.Warning("data file '%s' doesn't exist", fullPath)
	}

	fs.meta.Delete(m)

	return fuse.OK
}

func (fs *fileSystem) Symlink(pointedTo string, linkName string, context *fuse.Context) (code fuse.Status) {
	m, err := fs.meta.CreateFile(linkName)
	if err != nil {
		return fuse.ToStatus(err)
	}

	return fuse.ToStatus(m.Save(&meta.MetaData{
		Filetype:    syscall.S_IFLNK,
		Extended:    pointedTo,
		Permissions: 0777,
	}))
}

// Rename handles dir & file rename operation
func (fs *fileSystem) Rename(oldPath string, newPath string, context *fuse.Context) (codee fuse.Status) {
	fullOldPath := fs.GetPath(oldPath)
	fullNewPath := fs.GetPath(newPath)

	log.Debugf("Rename (%v) -> (%v)", oldPath, newPath)

	m, exists := fs.meta.Get(oldPath)
	if !exists {
		return fuse.ENOENT
	}

	// rename file
	if err := os.Rename(fullOldPath, fullNewPath); err != nil {
		log.Warning("data file doesn't exist")
	}

	// adjust metadata
	info, err := m.Load()
	if err != nil {
		return fuse.ToStatus(err)
	}

	fs.meta.Delete(m)

	nm, err := fs.meta.CreateFile(newPath)
	if err != nil {
		return fuse.ToStatus(err)
	}

	return fuse.ToStatus(nm.Save(info))
}

func (fs *fileSystem) Link(orig string, newName string, context *fuse.Context) (code fuse.Status) {
	return fuse.ToStatus(os.Link(fs.GetPath(orig), fs.GetPath(newName)))
}

func (fs *fileSystem) Access(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	log.Debugf("Access %v", fs.GetPath(name))
	return fuse.OK
	//return fuse.ToStatus(syscall.Access(fs.GetPath(name), mode))
}

func (fs *fileSystem) Create(name string, flags uint32, mode uint32, context *fuse.Context) (fuseFile nodefs.File, code fuse.Status) {
	log.Debugf("Create %v", name)

	dir := path.Dir(name)
	if _, ok := fs.meta.Get(dir); ok {
		os.MkdirAll(fs.GetPath(dir), 0755)
	} else {
		return nil, fuse.ENOENT
	}

	f, err := os.OpenFile(fs.GetPath(name), int(flags)|os.O_CREATE|os.O_TRUNC, os.FileMode(mode))
	if err != nil {
		return nil, fuse.EIO
	}

	m, err := fs.meta.CreateFile(name)
	if err != nil {
		return nil, fuse.ToStatus(err)
	}

	return NewLoopbackFile(m, f), fuse.OK
}

// download file from stor
func (fs *fileSystem) download(meta meta.Meta, path string) error {
	log.Infof("Downloading file '%s'", path)

	data, err := meta.Load()
	if err != nil {
		return err
	}

	body, err := fs.stor.Get(data.Hash)
	if err != nil {
		return err
	}

	defer body.Close()

	broReader, err := brotli.NewReader(body, nil)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()

	if fs.backend.Encrypted {
		if data.UserKey == "" {
			return fmt.Errorf("encryption key is empty, can't decrypt file %v", path)
		}

		r := bytes.NewBuffer([]byte(data.UserKey))
		bKey := []byte{}
		fmt.Fscanf(r, "%x", &bKey)

		sessionKey, err := crypto.DecryptAsym(fs.backend.ClientKey, bKey)
		if err != nil {
			log.Errorf("Error decrypting session key: %v", err)
			return err
		}

		if err := crypto.DecryptSym(sessionKey, broReader, file); err != nil {
			log.Errorf("Error decrypting data: %v", err)
			return err
		}
	} else {
		if _, err = io.Copy(file, broReader); err != nil {
			log.Errorf("Error downloading data: %v", err)
			_ = os.Remove(path)
			return err
		}
	}

	// setting locally file permission
	err = os.Chown(path, int(data.Uid), int(data.Gid))
	if err != nil {
		log.Errorf("Cannot chown %v to (%d, %d): %v", path, data.Uid, data.Gid, err)
	}

	// err = syscall.Chmod(path, 04755)
	err = syscall.Chmod(path, data.Permissions)
	if err != nil {
		log.Errorf("Cannot chmod %v to %d: %v", path, data.Permissions, err)
	}

	utbuf := &syscall.Utimbuf{
		Actime:  int64(data.Ctime),
		Modtime: int64(data.Mtime),
	}

	err = syscall.Utime(path, utbuf)
	if err != nil {
		log.Errorf("Cannot utime %v: %v", path, err)
	}

	return err
}

func (fs *fileSystem) Meta(path string) (*meta.MetaData, error) {
	m, exists := fs.meta.Get(path)
	if !exists {
		return nil, meta.ErrNotFound
	}

	return m.Load()
}
