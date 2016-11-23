package stream

import (
	"bufio"
	"github.com/op/go-logging"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var (
	log             = logging.MustGetLogger("stream")
	pmMsgPattern, _ = regexp.Compile("^(\\d+)(:{2,3})(.*)$")
)

type Consumer interface {
	Consume(MessageHandler)
	Signal() <-chan int
}

type consumerImpl struct {
	reader io.Reader
	level  int
	signal chan int
}

func NewConsumer(reader io.Reader, level int) Consumer {
	return &consumerImpl{
		reader: reader,
		level:  level,
		signal: make(chan int),
	}
}

// read input until the end (or closed)
// process all messages as speced x:: or x:::
// other messages that has no level are assumed of level consumer.level
func (consumer *consumerImpl) consume(handler MessageHandler) {
	reader := bufio.NewReader(consumer.reader)
	var level int
	var message string
	var multiline = false

	defer func() {
		consumer.signal <- 1
		close(consumer.signal)
	}()

	for {
		line, err := reader.ReadString('\n')

		if err != nil && err != io.EOF {
			log.Errorf("%s", err)
			return
		}

		line = strings.TrimRight(line, "\n")

		if line != "" {
			if !multiline {
				matches := pmMsgPattern.FindStringSubmatch(line)
				if matches == nil {
					//use default level.
					handler(&Message{
						Level:   consumer.level,
						Message: line,
					})
				} else {
					l, _ := strconv.ParseInt(matches[1], 10, 0)
					level = int(l)
					message = matches[3]

					if matches[2] == ":::" {
						multiline = true
					} else {
						//single line message
						handler(&Message{
							Level:   level,
							Message: message,
						})
					}
				}
			} else {
				/*
				   A known issue is that if stream was closed (EOF) before
				   we receive the ::: termination of multiline string. We discard
				   the uncomplete multiline string message.
				*/
				if line == ":::" {
					multiline = false
					//flush message
					handler(&Message{
						Level:   level,
						Message: message,
					})
				} else {
					message += "\n" + line
				}
			}
		}

		if err == io.EOF {
			return
		}
	}
}

func (consumer *consumerImpl) Consume(handler MessageHandler) {
	go consumer.consume(handler)
}

func (consumer *consumerImpl) Signal() <-chan int {
	return consumer.signal
}
