package rofs

import (
	"github.com/dsnet/compress/brotli"
	"github.com/g8os/g8ufs/meta"
	"io"
	"os"
	"path"
	"syscall"
)

func (fs *filesystem) path(hash string) string {
	return path.Join(fs.cache, hash)
}

func (fs *filesystem) exists(hash string) bool {
	name := path.Join(fs.cache, hash)
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

func (fs *filesystem) checkAndGet(m meta.Meta) (*os.File, error) {
	//atomic check and download a file
	name := fs.path(m.Hash())
	stat := m.Stat()
	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, os.ModePerm&os.FileMode(stat.Permissions))
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}

	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	fstat, err := f.Stat()

	if err != nil {
		return nil, err
	}

	if fstat.Size() == int64(stat.Size) {
		return f, nil
	}

	if err := fs.download(f, m.Hash()); err != nil {
		f.Close()
		os.Remove(name)
		return nil, err
	}

	f.Seek(0, os.SEEK_SET)
	return f, nil
}

// download file from stor
func (fs *filesystem) download(file *os.File, hash string) error {
	name := fs.path(hash)
	log.Infof("Downloading file '%s'", name)

	body, err := fs.storage.Get(hash)
	if err != nil {
		return err
	}

	defer body.Close()

	broReader, err := brotli.NewReader(body, nil)
	if err != nil {
		return err
	}

	if _, err = io.Copy(file, broReader); err != nil {
		log.Errorf("Error downloading data: %v", err)
		return err
	}

	return nil
}
