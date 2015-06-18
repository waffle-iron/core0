package pm


import (
    "io"
)

type InputConsumer struct {
    cmd *Cmd
    reader *io.Reader
    level int
}


func NewInputConsumer(cmd *Cmd, reader *io.Reader, level int) *InputConsumer{
    return &InputConsumer{
        cmd: cmd,
        reader: reader,
        level: level,
    }
}


func (consumer *InputConsumer) Consume() {
    // read input until the end (or closed)
    // process all messages as speced x:: or x:::
    // other messages that has no level are assumed of level consumer.level
}
