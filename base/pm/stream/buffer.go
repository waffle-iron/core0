package stream

import (
	"bytes"
	"container/list"
)

type Buffer interface {
	Append(string)
	String() string
}

type limitedBufferImpl struct {
	size   int
	buffer *list.List
}

func NewBuffer(size int) Buffer {
	return &limitedBufferImpl{
		size:   size,
		buffer: list.New(),
	}
}

func (buffer *limitedBufferImpl) String() string {
	var strbuf bytes.Buffer
	for l := buffer.buffer.Front(); l != nil; l = l.Next() {
		strbuf.WriteString(l.Value.(string))
		strbuf.WriteString("\n")
	}

	return strbuf.String()
}

func (buffer *limitedBufferImpl) Append(line string) {
	list := buffer.buffer
	list.PushBack(line)
	if list.Len() > buffer.size {
		list.Remove(list.Front())
	}
}
