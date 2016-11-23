package stats

import (
	"strconv"
)

type buffer interface {
	append(string)
	value() float64
	optype() string
}

type gauageBuffer struct {
	gauage float64
}

func newGauageBuffer() buffer {
	return new(gauageBuffer)
}

func (buffer *gauageBuffer) optype() string {
	return "g"
}

func (buffer *gauageBuffer) append(value string) {
	if value == "" {
		return
	}

	s := 1
	var op func(float64)

	switch value[0] {
	case '+':
		op = buffer.add
	case '-':
		op = buffer.sub
	default:
		s = 0
		op = buffer.set
	}

	num, err := strconv.ParseFloat(value[s:], 64)
	if err != nil {
		return
	}
	op(num)
}

func (buffer *gauageBuffer) add(v float64) {
	buffer.gauage += v
}

func (buffer *gauageBuffer) sub(v float64) {
	buffer.gauage -= v
}

func (buffer *gauageBuffer) set(v float64) {
	buffer.gauage = v
}

func (buffer *gauageBuffer) value() float64 {
	//gauage keeps last value on flush
	return buffer.gauage
}

type kvBuffer struct {
	count float64
}

func newKVBuffer() buffer {
	return new(kvBuffer)
}

func (buffer *kvBuffer) optype() string {
	return "kv"
}

func (buffer *kvBuffer) append(value string) {
	if value == "" {
		return
	}

	num, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return
	}

	buffer.count = num
}

func (buffer *kvBuffer) value() float64 {
	//reset to zero on retrieve.
	defer func() {
		buffer.count = 0
	}()
	return buffer.count
}

type counterBuffer struct {
	count float64
}

func newCounterBuffer() buffer {
	return new(counterBuffer)
}

func (buffer *counterBuffer) optype() string {
	return "c"
}

func (buffer *counterBuffer) append(value string) {
	if value == "" {
		return
	}

	num, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return
	}

	buffer.count += num
}

func (buffer *counterBuffer) value() float64 {
	//reset to zeron on retrieve
	defer func() {
		buffer.count = 0
	}()
	return buffer.count
}

type setBuffer struct {
	values map[string]bool
}

func newSetBuffer() buffer {
	return &setBuffer{
		values: make(map[string]bool),
	}
}

func (buffer *setBuffer) optype() string {
	return "s"
}

func (buffer *setBuffer) append(value string) {
	if value == "" {
		return
	}

	buffer.values[value] = true
}

func (buffer *setBuffer) value() float64 {
	//reset to zeron on retrieve
	defer func() {
		buffer.values = make(map[string]bool)
	}()

	return float64(len(buffer.values))
}

type timerBuffer struct {
	sum   float64
	count float64
}

func newTimerBuffer() buffer {
	return new(timerBuffer)
}

func (buffer *timerBuffer) optype() string {
	return "ms"
}

func (buffer *timerBuffer) append(value string) {
	if value == "" {
		return
	}

	num, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return
	}
	buffer.count++
	buffer.sum += num
}

func (buffer *timerBuffer) value() float64 {
	//reset to zeron on retrieve
	defer func() {
		buffer.count = 0
		buffer.sum = 0
	}()

	if buffer.count > 0 {
		return buffer.sum / buffer.count
	}

	return 0
}
