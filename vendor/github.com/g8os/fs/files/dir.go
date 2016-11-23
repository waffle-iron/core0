package files

import (
	"github.com/hanwen/go-fuse/fuse"
	"os"
)

// Mkdir creates a directory
func (fs *fileSystem) Mkdir(path string, mode uint32, context *fuse.Context) (code fuse.Status) {
	fullPath := fs.GetPath(path)

	log.Debugf("Mkdir %v", path)

	status := fuse.ToStatus(os.Mkdir(fullPath, os.FileMode(mode)))
	if status != fuse.OK {
		return status
	}

	fs.meta.CreateDir(path)
	// This line break mkdir on OL
	// fs.tracker.Touch(fullPath)
	return fuse.OK
}

// Rmdir deletes a directory
func (fs *fileSystem) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	fullPath := fs.GetPath(name)
	log.Debugf("Rmdir %v", fullPath)
	m, exists := fs.meta.Get(name)
	if !exists {
		return fuse.ENOENT
	}

	if err := os.Remove(fullPath); err != nil {
		return fuse.ToStatus(err)
	}

	return fuse.ToStatus(fs.meta.Delete(m))
}

// OpenDir opens a directory and return all files/dir in the directory.
// If it finds .meta file, it shows the file represented by that meta
func (fs *fileSystem) OpenDir(name string, context *fuse.Context) (stream []fuse.DirEntry, status fuse.Status) {
	log.Debugf("OpenDir %v", fs.GetPath(name))
	m, exists := fs.meta.Get(name)
	if !exists {
		return nil, fuse.ENOENT
	}
	var output []fuse.DirEntry
	log.Debugf("Listing children in directory %s", name)

	for child := range m.Children() {
		data, err := child.Load()
		if err != nil {
			return nil, fuse.ToStatus(err)
		}
		output = append(output, fuse.DirEntry{
			Name: child.Name(),
			Mode: data.Filetype,
		})
	}

	return output, fuse.OK
}
