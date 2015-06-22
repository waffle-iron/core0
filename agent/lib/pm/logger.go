package pm

import (
    "fmt"
    "log"
    "time"
)



var RESULT_MESSAGE []int = []int{20, 21, 22, 23, 30}

type Message struct {
    cmd *Cmd
    level int
    message string
}


func (msg Message) String() string {
    return fmt.Sprintf("%d:%s", msg.level, msg.message)
}

type Logger struct {
    queue_db chan *Message
    queue_ac chan *Message
    buffer []*Message
    flushInt time.Duration
    dbfactory DBFactory
}

func NewLogger(bufsize int, flushInt time.Duration, dbfactory DBFactory) *Logger {
    return &Logger{
        queue_db: make (chan *Message, 1024),
        queue_ac: make (chan *Message, 1024),
        buffer: make([]*Message, bufsize, bufsize),
        flushInt: flushInt,
        dbfactory: dbfactory,
    }
}

func in(l []int, x int) bool {
    for i := 0; i < len(l); i++ {
        if l[i] == x {
            return true
        }
    }

    return false
}

func (logger *Logger) Run() {
    go func() {
        for {
            msg := <- logger.queue_db
            //dump message directly to db
            db := logger.dbfactory.GetDBCon()
            stmnt := `
                insert into logs (id, domain, name, epoch, level, data)
                values (?, ?, ?, ?, ?, ?)
            `
            _, err := db.Exec(stmnt, 1, "testdomain", "testname",
                              time.Now().Unix(), msg.level, msg.message)
            if err != nil {
                log.Fatal(err)
            }
        }
    } ()
}

func (logger *Logger) Log(msg *Message) {
    if in(RESULT_MESSAGE, msg.level) {
        //send immediate to AC result
    }

    if !in(msg.cmd.args.GetLogLevels(), msg.level){
        //ignore, no further processing.
        return
    }

    if in(msg.cmd.args.GetLogLevelsDB(), msg.level) {
        //send to db worker.
        logger.queue_db <- msg
    }

    if in(msg.cmd.args.GetLogLevelsAC(), msg.level) {
        //send to ac logger.
        logger.queue_ac <- msg
    }
}
