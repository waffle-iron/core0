package pm


import (
    "io"
    "bufio"
    "log"
    "strings"
    "regexp"
    "strconv"
)

var PM_MESG_PATTERN, _ = regexp.Compile("^(\\d+)(:{2,3})(.*)$")

type StreamConsumer struct {
    cmd *Cmd
    reader io.Reader
    level int
    Signal chan int
}


func NewStreamConsumer(cmd *Cmd, reader io.Reader, level int) *StreamConsumer{
    return &StreamConsumer{
        cmd: cmd,
        reader: reader,
        level: level,
        Signal: make(chan int),
    }
}


func (consumer *StreamConsumer) Consume(handler MessageHandler) {
    // read input until the end (or closed)
    // process all messages as speced x:: or x:::
    // other messages that has no level are assumed of level consumer.level
    go func() {
        reader := bufio.NewReader(consumer.reader)
        var level int
        var message string
        var multiline bool = false

        for {
            line, err := reader.ReadString('\n')

            if err != nil && err != io.EOF {
                log.Println(err)
                return
            }

            line = strings.TrimRight(line, "\n")

            if line != "" {
                if !multiline {
                    matches := PM_MESG_PATTERN.FindStringSubmatch(line)
                    if matches == nil {
                        //use default level.
                        handler(&Message{
                            cmd: consumer.cmd,
                            level: consumer.level,
                            message: line,
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
                                cmd: consumer.cmd,
                                level: level,
                                message: message,
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
                            cmd: consumer.cmd,
                            level: level,
                            message: message,
                        })
                    } else {
                        message += "\n" + line
                    }
                }
            }

            if err == io.EOF {
                consumer.Signal <- 1
                close(consumer.Signal)
                return
            }
        }
    }()
}
