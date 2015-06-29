package stats

import (
    "time"
)


type msg struct {
    key string
    value float64
}

type FlushHandler func (*Stats)

type Stats struct {
    Timestamp int64 `json:"timestamp"`
    Series [][]interface{} `json:"series"`
}

type Statsd struct {
    flushInt time.Duration
    onflush FlushHandler
    buffer map[string][]float64
    queue chan msg
}

func NewStatsd(flush time.Duration, onflush FlushHandler) *Statsd {
    return &Statsd{
        flushInt: flush,
        onflush: onflush,
        buffer: make(map[string][]float64, 128),
        queue: make(chan msg),
    }
}


func (statsd *Statsd) Avg(key string, value float64) {
    //compute avg on flush
    statsd.queue <- msg{
        key: key,
        value: value,
    }
}

func (statsd *Statsd) flush() {
    //compute averages.
    flushSeconds := int64(statsd.flushInt / time.Second)
    timestamp := (time.Now().Unix() / flushSeconds) * flushSeconds

    stats := &Stats{
        Timestamp: int64(timestamp),
        Series: make([][]interface{}, len(statsd.buffer)),
    }

    i := 0
    for key, values := range statsd.buffer {
        var avg float64
        for _, v := range values {
            avg += v
        }

        avg = avg / float64(len(values))

        stats.Series[i] = []interface{}{key, avg}
        i += 1
    }

    if statsd.onflush != nil {
        statsd.onflush(stats)
    }

    statsd.buffer = make(map[string][]float64)
}

//starts the statsd routine
func (statsd *Statsd) Run() {
    go func () {
        var tick <- chan time.Time = time.After(statsd.flushInt)
        for {
            select {
            case msg := <-statsd.queue:
                values, ok := statsd.buffer[msg.key]
                if !ok {
                    values = make([]float64, 0, 10)
                }
                values = append(values, msg.value)
                statsd.buffer[msg.key] = values
            case <-tick:
                //reset timer
                tick = time.After(statsd.flushInt)
                statsd.flush()
            }
        }
    }()
}
