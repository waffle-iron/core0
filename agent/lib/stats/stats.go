package stats

import (
    "time"
    "fmt"
    "log"
    "strings"
    "errors"
)


const (
    TYPE_KEYVALUE = "kv"
    TYPE_GAUAGE = "g"
    TYPE_TIMER = "ms"
    TYPE_COUNTER = "c"
    TYPE_SET = "s"
)

type msg struct {
    optype string
    key string
    value string
    flag string
}

type FlushHandler func (*Stats)

type Stats struct {
    Timestamp int64 `json:"timestamp"`
    Series [][]interface{} `json:"series"`
}


type Statsd struct {
    prefix string
    flushInt time.Duration
    onflush FlushHandler
    buffer map[string]buffer
    queue chan msg
}

func NewStatsd(prefix string, flush time.Duration, onflush FlushHandler) *Statsd {
    return &Statsd{
        prefix: prefix,
        flushInt: flush,
        onflush: onflush,
        buffer: make(map[string]buffer),
        queue: make(chan msg),
    }
}


func (statsd *Statsd) op(optype string, key string, value string, flag string) {
    statsd.queue <- msg {
        optype: optype,
        key: key,
        value: value,
        flag: flag,
    }
}

/**
Feed statsd with statsD message according to specs
str is in format 'key:value|type[|@flag]'
*/
func (statsd *Statsd) Feed(str string) error {
    parts := strings.Split(str, "|")
    if len(parts) < 2 {
        return errors.New("Invalid statsd string, expecting data|type[|options]")
    }

    optype := parts[1]

    var flag string
    if len(parts) == 3 {
        flag = parts[2]
    }

    data := strings.Split(parts[0], ":")
    if len(data) != 2 {
        return errors.New("Invalid statsd data, expecting key:value")
    }

    key := data[0]
    value := data[1]

    statsd.op(optype, key, value, flag)

    return nil
}

//Gauage value
//value can be formatted as \d, +\d, -\d
func (statsd *Statsd) Gauage(key string, value string) {
    statsd.op(TYPE_GAUAGE, key, value, "")
}

func (statsd *Statsd) Counter(key string, value string) {
    statsd.op(TYPE_COUNTER, key, value, "")
}

func (statsd *Statsd) KeyValue(key string, value string) {
    statsd.op(TYPE_KEYVALUE, key, value, "")
}

func (statsd *Statsd) Timer(key string, value string) {
    statsd.op(TYPE_TIMER, key, value, "")
}

func (statsd *Statsd) Set(key string, value string) {
    statsd.op(TYPE_SET, key, value, "")
}

func (statsd *Statsd) flush() {
    //compute averages.
    if len(statsd.buffer) == 0 {
        //no data to flush
        return
    }

    flushSeconds := int64(statsd.flushInt / time.Second)
    timestamp := (time.Now().Unix() / flushSeconds) * flushSeconds

    stats := &Stats{
        Timestamp: int64(timestamp),
        Series: make([][]interface{}, len(statsd.buffer)),
    }

    i := 0
    for key, values := range statsd.buffer {
        key = fmt.Sprintf("%s.%s", statsd.prefix, key)

        value := values.value()

        stats.Series[i] = []interface{}{key, value}
        i += 1
    }

    if statsd.onflush != nil {
        statsd.onflush(stats)
    }

    // statsd.buffer = make(map[string]buffer)
}

//starts the statsd routine
func (statsd *Statsd) Run() {
    go func () {
        var tick <- chan time.Time = time.After(statsd.flushInt)
        loop:
        for {
            select {
            case msg, valid := <-statsd.queue:
                if !valid {
                    //closed queue.
                    break loop
                }
                buffer, ok := statsd.buffer[msg.key]
                if !ok {
                    switch msg.optype {
                    case TYPE_GAUAGE:
                        buffer = newGauageBuffer()
                    case TYPE_KEYVALUE:
                        buffer = newKVBuffer()
                    case TYPE_COUNTER:
                        buffer = newCounterBuffer()
                    case TYPE_SET:
                        buffer = newSetBuffer()
                    case TYPE_TIMER:
                        buffer = newTimerBuffer()
                    }
                    statsd.buffer[msg.key] = buffer
                }

                if buffer.optype() != msg.optype {
                    log.Println("Inconsistent aggregation operation on key", msg.key)
                    return
                }

                buffer.append(msg.value)
            case <-tick:
                //reset timer
                tick = time.After(statsd.flushInt)
                statsd.flush()
            }
        }
    }()
}

func (statsd *Statsd) Stop() {
    close(statsd.queue)
    //last flush
    statsd.flush()
}

