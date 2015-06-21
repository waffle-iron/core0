package stats

import (
    "time"
)


type msg struct {
    key string
    value float64
}

type FlushHandler func (key string, value float64)


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
    for key, values := range statsd.buffer {
        var avg float64
        for _, v := range values {
            avg += v
        }

        avg = avg / float64(len(values))
        if statsd.onflush != nil {
            statsd.onflush(key, avg)
        }
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
