package pm

import (
    "fmt"
    "log"
    "time"
    "encoding/json"
    "github.com/Jumpscale/jsagent/agent/lib/utils"
)

type Message struct {
    id uint32
    cmd *Cmd
    level int
    message string
    epoch int64
}

func (msg *Message) MarshalJSON() ([]byte, error) {
    data := make(map[string]interface{})
    args := msg.cmd.args
    data["domain"] = args.GetDomain()
    data["name"] = args.GetName()
    data["epoch"] = msg.epoch
    data["level"] = msg.level
    data["id"] = msg.id
    data["data"] = msg.message

    return json.Marshal(data)
}

func (msg *Message) String() string {
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
    args := msg.cmd.args
    _, err := db.Exec(stmnt, msg.id, args.GetDomain(), args.GetName(),
                      msg.epoch, msg.level, msg.message)
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
    s, err := json.Marshal(buffer)
    if err != nil {
        log.Println("Failed to serialize the logs")
    }

    log.Printf("%s\n", s)

    log.Println("Send batch to AC", len(buffer))
}
