package pm

import (
    "fmt"
    "log"
    "time"
    "github.com/Jumpscale/jsagent/agent/lib/utils"
)

type Message struct {
    cmd *Cmd
    level int
    message string
}


func (msg Message) String() string {
    return fmt.Sprintf("%d:%s", msg.level, msg.message)
}

type Logger interface {
    Log(msg *Message)
}

type DBLogger struct {
    factory DBFactory
}

func NewDBLogger(factory DBFactory) Logger {
    return &DBLogger{
        factory: factory,
    }
}

func (logger *DBLogger) Log(msg *Message) {
    if !utils.In(msg.cmd.args.GetLogLevelsDB(), msg.level) {
        return
    }

    db := logger.factory.GetDBCon()
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


type ACLogger struct {
    endpoint string
    buffer []*Message
    queue chan *Message
}

func NewACLogger(endpoint string, bufsize int, flushInt time.Duration) Logger {
    logger := &ACLogger {
        endpoint: endpoint,
        buffer: make([]*Message, 0, bufsize),
        queue: make(chan *Message),
    }

    go func() {
        //autostart logger flusher.
        for {
            select {
            case msg := <- logger.queue:
                if len(logger.buffer) < cap(logger.buffer) {
                    logger.buffer = append(logger.buffer, msg)
                }

                if len(logger.buffer) == cap(logger.buffer) {
                    //no more buffer space.
                    logger.flush()
                }
            case <- time.After(flushInt):
                logger.flush()
            }
        }
    } ()

    return logger
}

func (logger *ACLogger) Log(msg *Message) {
    if !utils.In(msg.cmd.args.GetLogLevelsAC(), msg.level) {
        return
    }
    logger.queue <- msg
}

func (logger *ACLogger) flush() {
    basket := make([]*Message, len(logger.buffer))
    copy(basket, logger.buffer)
    go logger.send(basket)

    logger.buffer = logger.buffer[0:0]
}

func (logger *ACLogger) send(buffer []*Message) {
    log.Println("Send batch to AC", len(buffer))
}
