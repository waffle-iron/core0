package meta

import (
	"os/user"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
)

type memMeta struct {
	path     string
	hash     string
	meta     MetaData
	children map[string]*memMeta
}

func (m *memMeta) Hash() string {
	return m.hash
}

func (m *memMeta) String() string {
	return m.path
}

func (m *memMeta) Name() string {
	return path.Base(m.path)
}

func (m *memMeta) Stat() MetaData {
	return m.meta
}

func (m *memMeta) Children() <-chan Meta {
	ch := make(chan Meta)
	go func() {
		defer close(ch)
		for _, child := range m.children {
			ch <- child
		}
	}()

	return ch
}

type memMetaStore struct {
	root *memMeta
	ino  uint64
}

func NewMemoryMetaStore() MetaStore {
	return &memMetaStore{
		root: &memMeta{
			meta: MetaData{
				IsDir:       true,
				Permissions: 0755,
				Gid:         0,
				Uid:         0,
			},
		},
	}
}

func (s *memMetaStore) mkall(name string) *memMeta {
	name = strings.Trim(name, "/")
	parts := strings.Split(path.Clean(name), "/")
	m := s.root
	for i, part := range parts {
		if part == "." {
			continue
		}

		if m.children == nil {
			m.children = make(map[string]*memMeta)
		}

		c, ok := m.children[part]
		if ok {
			m = c
			continue
		}

		c = &memMeta{
			path: path.Join(parts[0 : i+1]...),
		}

		if i != len(parts)-1 {
			//intermediate directory
			c.meta = MetaData{
				IsDir:       true,
				Permissions: 0755,
				Uid:         0,
				Gid:         0,
				Inode:       atomic.AddUint64(&s.ino, 1),
			}
		}

		m.children[part] = c
		m = c
	}

	return m
}

func (s *memMetaStore) delete(name string) {
	name = strings.Trim(name, "/")
	parts := strings.Split(path.Clean(name), "/")
	m := s.root
	for i, part := range parts {
		if part == "." {
			continue
		}

		if m.children == nil {
			return
		}

		if i == len(parts)-1 {
			delete(m.children, part)
		}
		c, ok := m.children[part]
		if !ok {
			return
		}
		m = c
	}
}

func (s *memMetaStore) Get(name string) (Meta, bool) {
	name = strings.Trim(name, "/")
	parts := strings.Split(path.Clean(name), "/")
	m := s.root
	for _, part := range parts {
		if part == "." {
			continue
		}

		c, ok := m.children[part]
		if !ok {
			return nil, false
		}
		m = c
	}

	return m, true
}

func (s *memMetaStore) Populate(entry Entry) error {

	// user and group id
	uid := 0
	u, err := user.Lookup(entry.Uname)
	if err == nil {
		uid, _ = strconv.Atoi(u.Uid)
	}

	gid := 0
	g, err := user.LookupGroup(entry.Gname)
	if err == nil {
		gid, _ = strconv.Atoi(g.Gid)
	}

	isDir := false
	if entry.Filetype == syscall.S_IFDIR {
		isDir = true
	}

	meta := s.mkall(entry.Filepath)
	meta.hash = entry.Hash
	meta.meta = MetaData{
		Hash:        entry.Hash,
		Size:        uint64(entry.Filesize),
		Uname:       entry.Uname,
		Uid:         uint32(uid),
		Gname:       entry.Gname,
		Gid:         uint32(gid),
		Permissions: uint32(entry.Permissions),
		IsDir:       isDir,
		Ctime:       uint64(entry.Ctime.Unix()),
		Mtime:       uint64(entry.Mtime.Unix()),
		Inode:       atomic.AddUint64(&s.ino, 1),
	}

	return nil
}
