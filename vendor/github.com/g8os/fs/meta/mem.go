package meta

import (
	"github.com/g8os/fs/utils"
	"os/user"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
)

type memMeta struct {
	path     string
	meta     *MetaData
	stat     MetaState
	children map[string]*memMeta
}

func (m *memMeta) String() string {
	return m.path
}

func (m *memMeta) Name() string {
	return path.Base(m.path)
}

func (m *memMeta) Stat() MetaState {
	return m.stat
}

func (m *memMeta) SetStat(state MetaState) {
	m.stat = state
}

func (m *memMeta) Load() (*MetaData, error) {
	return m.meta, nil
}

func (m *memMeta) Save(meta *MetaData) error {
	m.meta = meta
	return nil
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
			meta: &MetaData{
				Filetype:    syscall.S_IFDIR,
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
			c.meta = &MetaData{
				Filetype:    syscall.S_IFDIR,
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

func (s *memMetaStore) Populate(plist string, trim string) error {
	iter, err := utils.IterFlistFile(plist)
	if err != nil {
		return err
	}

	for line := range iter {
		entity, err := ParseLine(line, trim)
		if err != nil {
			return err
		}

		// user and group id
		uid := 0
		u, err := user.Lookup(entity.Uname)
		if err == nil {
			uid, _ = strconv.Atoi(u.Uid)
		}

		gid := 0
		g, err := user.LookupGroup(entity.Gname)
		if err == nil {
			gid, _ = strconv.Atoi(g.Gid)
		}

		meta := s.mkall(entity.Filepath)
		meta.meta = &MetaData{
			Hash:        entity.Hash,
			Size:        uint64(entity.Filesize),
			Uname:       entity.Uname,
			Uid:         uint32(uid),
			Gname:       entity.Gname,
			Gid:         uint32(gid),
			Permissions: uint32(entity.Permissions),
			Filetype:    entity.Filetype,
			Ctime:       uint64(entity.Ctime.Unix()),
			Mtime:       uint64(entity.Mtime.Unix()),
			Extended:    entity.Extended,
			DevMajor:    entity.DevMajor,
			DevMinor:    entity.DevMinor,
			Inode:       atomic.AddUint64(&s.ino, 1),
		}
	}
	log.Debugf("Populated: %d", s.ino)

	return nil
}

func (s *memMetaStore) CreateFile(name string) (Meta, error) {
	m := s.mkall(name)
	m.meta = &MetaData{
		Filetype: syscall.S_IFREG,
	}
	return m, nil
}

func (s *memMetaStore) CreateDir(name string) (Meta, error) {
	m := s.mkall(name)
	m.meta = &MetaData{
		Filetype: syscall.S_IFDIR,
	}
	return m, nil
}

func (s *memMetaStore) Delete(meta Meta) error {
	s.delete(meta.String())
	return nil
}
