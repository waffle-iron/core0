package pm

import (
    "fmt"
    "log"
    "time"
    "encoding/json"
    "bytes"
    "net/http"
    "github.com/Jumpscale/jsagent/agent/lib/utils"
)

const (
    L_STDOUT = 1  // stdout
    L_STDERR = 2  // stderr
    L_PUBLIC = 3  // message for endusers / public message
    L_OPERATOR = 4  // message for operator / internal message
    L_UNKNOWN = 5  // log msg (unstructured = level5, cat=unknown)
    L_STRUCTURED = 6  // log msg structured
    L_WARNING = 7  // warning message
    L_OPS_ERROR = 8  // ops error
    L_CRITICAL = 9  // critical error
    L_STATSD = 10  // statsd message(s)
    L_RESULT_JSON = 20  // result message, json
    L_RESULT_YAML = 21  // result message, yaml
    L_RESULT_TOML = 22  // result message, toml
    L_RESULT_HRD = 23  // result message, hrd
    L_RESULT_JOB = 30  // job, json (full result of a job)
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
    data["domain"] = args.GetString("domaing")
    data["name"] = args.GetString("name")
    data["epoch"] = msg.epoch
    data["level"] = msg.level
    data["id"] = msg.id
    data["data"] = msg.message

    return json.Marshal(data)
}

func (msg *Message) String() string {
    return fmt.Sprintf("%s|%d:%s", msg.cmd, msg.level, msg.message)
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
    if !utils.In(msg.cmd.args.GetIntArray("loglevels_db"), msg.level) {
        return
    }

    db := logger.factory.GetDBCon()
    stmnt := `
        insert into logs (id, domain, name, epoch, level, data)
        values (?, ?, ?, ?, ?, ?)
    `
    args := msg.cmd.args
    _, err := db.Exec(stmnt, msg.id, args.GetString("domain"), args.GetString("name"),
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
    if !utils.In(msg.cmd.args.GetIntArray("loglevels_ac"), msg.level) {
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
    if len(buffer) == 0 {
        //buffer can be of length zero, when flushed on timer while
        //no messages are ready.
        return
    }

    msgs, err := json.Marshal(buffer)
    if err != nil {
        log.Println("Failed to serialize the logs")
        return
    }

    reader := bytes.NewReader(msgs)
    resp, err := http.Post(logger.endpoint, "application/json", reader)
    if err != nil {
        log.Println("Failed to send log batch to AC", err)
        return
    }
    resp.Body.Close()
}
