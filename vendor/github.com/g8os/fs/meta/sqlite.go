package meta

import (
	"database/sql"
	"github.com/g8os/fs/utils"
	_ "github.com/mattn/go-sqlite3"
	"os/user"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	INSERT_STMT = `insert into meta (inode, parent, path, state, hash, uid, gid, permissions, filetype, ctime, mtime, extended, devmajor, devminor) values
		(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
)

type sqlMeta struct {
	path string
	meta *MetaData
	stat MetaState
	db   *sql.DB
}

func (m *sqlMeta) String() string {
	return m.path
}

func (m *sqlMeta) Name() string {
	return path.Base(m.path)
}

func (m *sqlMeta) Stat() MetaState {
	return m.stat
}

func (m *sqlMeta) SetStat(state MetaState) {
	m.stat = state
}

func (m *sqlMeta) Load() (*MetaData, error) {
	return m.meta, nil
}

func (m *sqlMeta) Save(meta *MetaData) error {
	m.meta = meta
	return nil
}

func (m *sqlMeta) Children() <-chan Meta {
	rows, err := m.db.Query(`select inode, path, hash, state, uid, gid, permissions, filetype, ctime, mtime, extended, devmajor, devminor from meta
		where parent = ?`, m.path)

	if err != nil {
		return nil
	}

	ch := make(chan Meta)
	go func() {
		defer rows.Close()
		defer close(ch)
		for rows.Next() {
			state := MetaInitial
			name := ""
			meta := MetaData{}
			if err := rows.Scan(&meta.Inode, &name, &meta.Hash, &state, &meta.Uid, &meta.Gid, &meta.Permissions, &meta.Filetype, &meta.Ctime, &meta.Mtime, &meta.Extended, &meta.DevMajor, &meta.DevMinor); err != nil {
				break
			}

			ch <- &sqlMeta{
				db:   m.db,
				path: name,
				meta: &meta,
				stat: state,
			}
		}
	}()

	return ch
}

type sqliteMetaStore struct {
	db  *sql.DB
	ino uint64
}

func NewSqliteMetaStore(name string) (MetaStore, error) {
	db, err := sql.Open("sqlite3", name)
	if err != nil {
		return nil, err
	}

	/*
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
	*/
	_, err = db.Exec(`
	create table meta (inode not null primary key, parent text, path text unique, state int64, hash text,
	uid int, gid int, permissions int, filetype int, ctime int, mtime int, extended text, devmajor int64, devminor int64);

	delete from meta;
	`)

	if err != nil {
		return nil, err
	}

	return &sqliteMetaStore{
		db: db,
	}, nil
}

func (s *sqliteMetaStore) parentExists(parent string) bool {
	row := s.db.QueryRow(`select count(inode) from meta where parent = ?`, parent)
	var count int
	if err := row.Scan(&count); err != nil {
		log.Errorf("error: %s", err)
		return false
	}

	return count > 0
}

func (s *sqliteMetaStore) Get(name string) (Meta, bool) {
	name = strings.Trim(path.Clean(name), "/")

	if name == "." {
		return &sqlMeta{
			db:   s.db,
			path: name,
			meta: &MetaData{
				Filetype:    syscall.S_IFDIR,
				Permissions: 0755,
			},
			stat: MetaInitial,
		}, true
	}

	row := s.db.QueryRow(`select inode, hash, state, uid, gid, permissions, filetype, ctime, mtime, extended, devmajor, devminor from meta
		where path = ?`, name)

	state := MetaInitial
	m := MetaData{}
	if err := row.Scan(&m.Inode, &m.Hash, &state, &m.Uid, &m.Gid, &m.Permissions, &m.Filetype, &m.Ctime, &m.Mtime, &m.Extended, &m.DevMajor, &m.DevMinor); err != nil {
		log.Errorf("sql error: %s", err)
		return nil, false
	}

	return &sqlMeta{
		db:   s.db,
		path: name,
		meta: &m,
		stat: state,
	}, true
}

func (s *sqliteMetaStore) CreateFile(name string) (Meta, error) {
	if meta, ok := s.Get(name); ok {
		return meta, nil
	}
	name = path.Clean(name)
	name = strings.Trim(name, "/")

	parent := path.Dir(name)

	m := &sqlMeta{
		db:   s.db,
		path: name,
		stat: MetaInitial,
		meta: &MetaData{
			Filetype:    syscall.S_IFREG,
			Permissions: 0744,
		},
	}
	_, err := s.db.Exec(INSERT_STMT,
		atomic.AddUint64(&s.ino, 1),
		parent,
		name,
		m.stat,
		"",
		0,
		0,
		m.meta.Permissions,
		m.meta.Filetype,
		time.Now().Unix(),
		time.Now().Unix(),
		"",
		0,
		0,
	)

	return m, err
}

func (s *sqliteMetaStore) CreateDir(name string) (Meta, error) {
	if meta, ok := s.Get(name); ok {
		return meta, nil
	}
	name = path.Clean(name)
	name = strings.Trim(name, "/")

	parent := path.Dir(name)

	m := &sqlMeta{
		db:   s.db,
		path: name,
		stat: MetaInitial,
		meta: &MetaData{
			Filetype:    syscall.S_IFDIR,
			Permissions: 0755,
		},
	}

	_, err := s.db.Exec(INSERT_STMT,
		atomic.AddUint64(&s.ino, 1),
		parent,
		name,
		m.stat,
		"",
		0,
		0,
		m.meta.Permissions,
		m.meta.Filetype,
		time.Now().Unix(),
		time.Now().Unix(),
		"",
		0,
		0,
	)

	return m, err
}

func (s *sqliteMetaStore) Delete(meta Meta) error {
	_, err := s.db.Exec(`delete from meta where path = ?`, meta.String())
	if err != nil {
		return err
	}
	return nil
}

func (s *sqliteMetaStore) Populate(plist string, trim string) error {
	log.Debugf("Populating plist")
	iter, err := utils.IterFlistFile(plist)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(INSERT_STMT)

	if err != nil {
		return err
	}

	parents := map[string]int{}

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

		name := strings.Trim(path.Clean(entity.Filepath), "/")

		parent := path.Dir(name)

		base := parent
		for base != "." {
			parents[base] = 1
			base = path.Dir(base)
		}

		stmt.Exec(
			atomic.AddUint64(&s.ino, 1),
			parent,
			name,
			MetaInitial,
			entity.Hash,
			uid,
			gid,
			entity.Permissions,
			entity.Filetype,
			uint64(entity.Ctime.Unix()),
			uint64(entity.Mtime.Unix()),
			entity.Extended,
			entity.DevMajor,
			entity.DevMinor,
		)
	}

	ux := time.Now().Unix()
	for parent := range parents {
		_, err := stmt.Exec(
			atomic.AddUint64(&s.ino, 1),
			path.Dir(parent),
			parent,
			MetaInitial,
			"",
			0,
			0,
			0755,
			syscall.S_IFDIR,
			ux,
			ux,
			"",
			0,
			0,
		)

		if err != nil {
			return err
		}
	}

	tx.Commit()
	log.Debugf("Populated: %d", s.ino)

	return nil
}
