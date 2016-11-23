package stats

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/op/go-logging"
	"strings"
	"time"
)

var (
	log = logging.MustGetLogger("stats")
)

const (
	//TypeKeyValue key value aggregator
	TypeKeyValue = "kv"
	//TypeGauage gauage aggregator
	TypeGauage = "g"
	//TypeTimer timer aggregator
	TypeTimer = "ms"
	//TypeCounter counter aggregator
	TypeCounter = "c"
	//TypeSet set aggregator
	TypeSet = "s"
)

type msg struct {
	optype string
	key    string
	value  string
	flag   string
}

//FlushHandler is a callback type for stats flush
type FlushHandler func(*Stats)

//Stats is the aggregated stats
type Stats struct {
	Timestamp int64           `json:"timestamp"`
	Series    [][]interface{} `json:"series"`
}

func (s *Stats) String() string {
	data, _ := json.Marshal(s)
	return string(data)
}

//Statsd represents the statsd daemon
type Statsd struct {
	prefix   string
	flushInt time.Duration
	onflush  FlushHandler
	buffer   map[string]buffer
	queue    chan msg
}

//NewStatsd creats a new statsd daemon
func NewStatsd(prefix string, flush time.Duration, onflush FlushHandler) *Statsd {
	return &Statsd{
		prefix:   prefix,
		flushInt: flush,
		onflush:  onflush,
		buffer:   make(map[string]buffer),
		queue:    make(chan msg),
	}
}

func (statsd *Statsd) op(optype string, key string, value string, flag string) {
	statsd.queue <- msg{
		optype: optype,
		key:    key,
		value:  value,
		flag:   flag,
	}
}

/*
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

	key := strings.Trim(data[0], " ")
	value := data[1]

	statsd.op(optype, key, value, flag)

	return nil
}

//Gauage value change value, keep last on flush
func (statsd *Statsd) Gauage(key string, value string) {
	statsd.op(TypeGauage, key, value, "")
}

//Counter increments value, resets on flush
func (statsd *Statsd) Counter(key string, value string) {
	statsd.op(TypeCounter, key, value, "")
}

//KeyValue updates value to last, reset on flus
func (statsd *Statsd) KeyValue(key string, value string) {
	statsd.op(TypeKeyValue, key, value, "")
}

//Timer calcluates mean, median on time
func (statsd *Statsd) Timer(key string, value string) {
	statsd.op(TypeTimer, key, value, "")
}

//Set keeps the count of unique values.
func (statsd *Statsd) Set(key string, value string) {
	statsd.op(TypeSet, key, value, "")
}

func (statsd *Statsd) flush() {
	//compute averages.
	if len(statsd.buffer) == 0 {
		//no data to flush
		return
	}

	flushSeconds := int64(statsd.flushInt / time.Second)
	if flushSeconds <= 0 {
		log.Errorf("Invalid stats flush internval: %d", flushSeconds)
		return
	}

	timestamp := (time.Now().Unix() / flushSeconds) * flushSeconds

	stats := &Stats{
		Timestamp: int64(timestamp),
		Series:    make([][]interface{}, len(statsd.buffer)),
	}

	i := 0
	for key, values := range statsd.buffer {
		key = fmt.Sprintf("%s.%s", statsd.prefix, key)

		value := values.value()

		stats.Series[i] = []interface{}{key, value}
		i++
	}

	if statsd.onflush != nil {
		statsd.onflush(stats)
	}

	// statsd.buffer = make(map[string]buffer)
}

//Run starts the statsd routine
func (statsd *Statsd) Run() {
	go func() {
		var tick = time.After(statsd.flushInt)
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
					case TypeGauage:
						buffer = newGauageBuffer()
					case TypeKeyValue:
						buffer = newKVBuffer()
					case TypeCounter:
						buffer = newCounterBuffer()
					case TypeSet:
						buffer = newSetBuffer()
					case TypeTimer:
						buffer = newTimerBuffer()
					}
					statsd.buffer[msg.key] = buffer
				}

				if buffer.optype() != msg.optype {
					log.Errorf("Inconsistent aggregation operation on key %s", msg.key)
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

//Stop stops the stats rountine and force flushing
func (statsd *Statsd) Stop() {
	close(statsd.queue)
	//last flush
	statsd.flush()
}
