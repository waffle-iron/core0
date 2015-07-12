package stats

import (
    "time"
    "fmt"
    "math"
    "log"
)


const (
    OP_AVG = 1
    OP_MAX = 2
    OP_MIN = 3
)

type msg struct {
    op int
    key string
    value float64
}

type FlushHandler func (*Stats)

type Stats struct {
    Timestamp int64 `json:"timestamp"`
    Series [][]interface{} `json:"series"`
}

type buffer interface {
    append(float64)
    value() float64
    op() int
}

type basicBuffer struct {
    values []float64
}

func (buffer *basicBuffer) append(value float64) {
    buffer.values = append(buffer.values, value)
}

type avgBuffer struct {
    basicBuffer
}

type maxBuffer struct {
    basicBuffer
}

type minBuffer struct {
    basicBuffer
}

func newAvgBuffer() buffer {
    return &avgBuffer{
        basicBuffer{
            values: make([]float64, 0, 10),
        },
    }
}

func (buffer *avgBuffer) value() float64 {
    var avg float64
    for _, v := range buffer.values {
        avg += v
    }

    avg = avg / float64(len(buffer.values))
    return avg
}

func (buffer *avgBuffer) op() int {
    return OP_AVG
}

func newMaxBuffer() buffer {
    return &maxBuffer{
        basicBuffer{
            values: make([]float64, 0, 10),
        },
    }
}

func (buffer *maxBuffer) value() float64 {
    var max float64
    for _, v := range buffer.values {
        if v > max {
            max = v
        }
    }

    return max
}

func (buffer *maxBuffer) op() int {
    return OP_MAX
}

func newMinBuffer() buffer {
    return &minBuffer{
        basicBuffer{
            values: make([]float64, 0, 10),
        },
    }
}

func (buffer *minBuffer) value() float64 {
    var min float64 = math.MaxFloat64
    for _, v := range buffer.values {
        if v < min {
            min = v
        }
    }

    return min
}

func (buffer *minBuffer) op() int {
    return OP_MIN
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


func (statsd *Statsd) op(op int, key string, value float64) {
    //compute avg on flush
    statsd.queue <- msg{
        op: op,
        key: key,
        value: value,
    }
}

func (statsd *Statsd) Avg(key string, value float64) {
    statsd.op(OP_AVG, key, value)
}

func (statsd *Statsd) Max(key string, value float64) {
    statsd.op(OP_MAX, key, value)
}

func (statsd *Statsd) Min(key string, value float64) {
    statsd.op(OP_MIN, key, value)
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

    statsd.buffer = make(map[string]buffer)
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
                values, ok := statsd.buffer[msg.key]
                if !ok {
                    switch msg.op {
                    case OP_AVG:
                        values = newAvgBuffer()
                    case OP_MAX:
                        values = newMaxBuffer()
                    case OP_MIN:
                        values = newMinBuffer()
                    }
                }

                if values.op() != msg.op {
                    log.Println("Inconsistent aggregation operation on key", msg.key)
                    return
                }

                values.append(msg.value)
                statsd.buffer[msg.key] = values
            case <-tick:
                //reset timer
                tick = time.After(statsd.flushInt)
                statsd.flush()
            }
        }
        log.Println("Exit statsd for job")
    }()
}

func (statsd *Statsd) Stop() {
    close(statsd.queue)
    //last flush
    statsd.flush()
}

