package pm

import (
    "fmt"
)

type Message struct {
    level int
    message string
}


func (msg Message) String() string {
    return fmt.Sprintf("%d:%s", msg.level, msg.message)
}
