package files

import (
	"fmt"
	"os"
	"sync"
	"syscall"

	"github.com/g8os/fs/meta"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

// LoopbackFile delegates all operations back to an underlying os.File.
func NewLoopbackFile(m meta.Meta, f *os.File) nodefs.File {
	return &loopbackFile{
		File: f,
		m:    m,
	}
}

type loopbackFile struct {
	File *os.File
	m    meta.Meta
	// os.File is not threadsafe. Although fd themselves are
	// constant during the lifetime of an open file, the OS may
	// reuse the fd number after it is closed. When open races
	// with another close, they may lead to confusion as which
	// file gets written in the end.
	lock sync.Mutex
}

func (f *loopbackFile) InnerFile() nodefs.File {
	return nil
}

func (f *loopbackFile) SetInode(n *nodefs.Inode) {
}

func (f *loopbackFile) String() string {
	return fmt.Sprintf("loopbackFile(%s)", f.File.Name())
}

func (f *loopbackFile) Read(buf []byte, off int64) (res fuse.ReadResult, code fuse.Status) {
	f.lock.Lock()
	defer f.lock.Unlock()
	// This is not racy by virtue of the kernel properly
	// synchronizing the open/write/close.
	r := fuse.ReadResultFd(f.File.Fd(), off, len(buf))
	return r, fuse.OK
}

func (f *loopbackFile) Write(data []byte, off int64) (uint32, fuse.Status) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.m.SetStat(f.m.Stat().SetModified(true))
	n, err := f.File.WriteAt(data, off)
	return uint32(n), fuse.ToStatus(err)
}

func (f *loopbackFile) Release() {
	log.Debugf("Release file %v", f.File.Name())
	f.lock.Lock()
	defer f.lock.Unlock()
	f.File.Close()
}

func (f *loopbackFile) Flush() fuse.Status {
	f.lock.Lock()

	// Since Flush() may be called for each dup'd fd, we don't
	// want to really close the file, we just want to flush. This
	// is achieved by closing a dup'd fd.
	newFd, err := syscall.Dup(int(f.File.Fd()))
	f.lock.Unlock()

	if err != nil {
		return fuse.ToStatus(err)
	}
	err = syscall.Close(newFd)
	return fuse.ToStatus(err)
}

func (f *loopbackFile) Fsync(flags int) (code fuse.Status) {
	f.lock.Lock()
	defer f.lock.Unlock()
	r := fuse.ToStatus(syscall.Fsync(int(f.File.Fd())))

	return r
}

func (f *loopbackFile) Truncate(size uint64) fuse.Status {
	f.lock.Lock()
	defer f.lock.Unlock()
	r := fuse.ToStatus(syscall.Ftruncate(int(f.File.Fd()), int64(size)))

	return r
}

func (f *loopbackFile) Chmod(mode uint32) fuse.Status {
	f.lock.Lock()
	defer f.lock.Unlock()
	r := fuse.ToStatus(f.File.Chmod(os.FileMode(mode)))

	return r
}

func (f *loopbackFile) Chown(uid uint32, gid uint32) fuse.Status {
	f.lock.Lock()
	defer f.lock.Unlock()

	r := fuse.ToStatus(f.File.Chown(int(uid), int(gid)))

	return r
}

func (f *loopbackFile) GetAttr(a *fuse.Attr) fuse.Status {
	log.Debugf("GetAttr file %v", f.File.Name())

	st := syscall.Stat_t{}
	f.lock.Lock()
	err := syscall.Fstat(int(f.File.Fd()), &st)
	f.lock.Unlock()
	if err != nil {
		return fuse.ToStatus(err)
	}
	a.FromStat(&st)

	return fuse.OK
}
