package utils

import (
	"time"
)

type BufferFlush func([]interface{})

type Buffer interface {
	Append(obj interface{})
}

type TimedBuffer struct {
	array   []interface{}
	queue   chan interface{}
	onflush BufferFlush
}

func NewBuffer(capacity int, flushInt time.Duration, onflush BufferFlush) Buffer {
	buffer := &TimedBuffer{
		array:   make([]interface{}, 0, capacity),
		queue:   make(chan interface{}),
		onflush: onflush,
	}

	go func() {
		//autostart buffer flusher.
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
			case <-time.After(flushInt):
				buffer.flush()
			}
		}
	}()

	return buffer
}

func (buffer *TimedBuffer) Append(obj interface{}) {
	buffer.queue <- obj
}

func (buffer *TimedBuffer) flush() {
	basket := make([]interface{}, len(buffer.array))
	copy(basket, buffer.array)
	go buffer.onflush(basket)

	buffer.array = buffer.array[0:0]
}
