package utils

import (
	"time"
)

//BufferFlush callback type for on flush event
type BufferFlush func([]interface{})

//Buffer interface
type Buffer interface {
	Append(obj interface{})
}

//TimedBuffer flushes buffer on time or if max buffer size is reached
type TimedBuffer struct {
	array   []interface{}
	queue   chan interface{}
	onflush BufferFlush
}

//NewBuffer creates a new timed buffer
func NewBuffer(capacity int, flushInt time.Duration, onflush BufferFlush) Buffer {
	buffer := &TimedBuffer{
		array:   make([]interface{}, 0, capacity),
		queue:   make(chan interface{}),
		onflush: onflush,
	}

	go func() {
		//autostart buffer flusher.
		timeout := time.After(flushInt)
		for {
			select {
			case msg := <-buffer.queue:
				if len(buffer.array) < capacity {
					buffer.array = append(buffer.array, msg)
				}

				if len(buffer.array) >= capacity {
					//no more buffer space.
					buffer.flush()
				}
			case <-timeout:
				timeout = time.After(flushInt)
				buffer.flush()
			}
		}
	}()

	return buffer
}

//Append appends object to buffer
func (buffer *TimedBuffer) Append(obj interface{}) {
	buffer.queue <- obj
}

func (buffer *TimedBuffer) flush() {
	basket := make([]interface{}, len(buffer.array))
	copy(basket, buffer.array)
	go buffer.onflush(basket)

	buffer.array = buffer.array[0:0]
}
