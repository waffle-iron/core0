package pm

import (
    "os"
    "time"
    "path"
    "fmt"
    "log"
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)

const (
    RECYCLE_SIZE = 100 * 1024 * 1024
)


type DBFactory interface {
    GetDBCon() *sql.DB
}

type SqliteDBFactory struct {
    db *sql.DB
    dir string
    getcount int
}

func NewSqliteFactory(dir string) DBFactory {
    return &SqliteDBFactory{
        db: nil,
        dir: dir,
    }
}

func (slite *SqliteDBFactory) GetDBCon() *sql.DB {
    if slite.db == nil {
        stat, err := os.Stat(slite.getDBFilePath("current"))

        if os.IsNotExist(err) {
            //init db.
            slite.db = slite.initDB()
        } else if stat.Size() >= RECYCLE_SIZE {
            //move old file
            if slite.db != nil {
                slite.db.Close()
            }
            os.Rename(
                slite.getDBFilePath("current"),
                slite.getDBFilePath(fmt.Sprintf("%d", time.Now().Unix())))
            slite.db = slite.initDB()
        } else {
            db, err := sql.Open("sqlite3", slite.getDBFilePath("current"))
            if err != nil {
                log.Fatal("Couldn't open db connection", err)
            }
            slite.db = db
        }
    } else {
        if slite.getcount >= 10000 {
            //recycle.
            slite.db.Close()
            slite.db = nil
            slite.getcount = 0
            slite.GetDBCon()
        }
    }

    slite.getcount += 1
    return slite.db
}

func (slite *SqliteDBFactory) initDB() *sql.DB {
    db, err := sql.Open("sqlite3", slite.getDBFilePath("current"))
    if err != nil {
        log.Fatal("Failed to open db connection", err)
    }

    _, err = db.Exec(`
        create table logs (
            id integer not null,
            domain text,
            name text,
            epoch integer,
            level integer,
            data text
        )
    `)

    if err != nil {
        log.Fatal(err)
    }

    return db
}

func (slite *SqliteDBFactory) getDBFilePath(name string) string {
    return path.Join(slite.dir, fmt.Sprintf("%s.db", name))
}
