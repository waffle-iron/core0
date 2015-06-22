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

type DBFactory interface {
    GetDBCon() *sql.DB
}

type SqliteDBFactory struct {
    db *sql.DB
    dir string
}

func NewSqliteFactory(dir string) DBFactory {
    return &SqliteDBFactory{
        db: nil,
        dir: dir,
    }
}

func (slite *SqliteDBFactory) GetDBCon() *sql.DB {
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
    } else if slite.db == nil {
        db, err := sql.Open("sqlite3", slite.getDBFilePath("current"))
        if err != nil {
            log.Fatal("Couldn't open db connection", err)
        }
        slite.db = db
    }

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
