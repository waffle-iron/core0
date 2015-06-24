package pm

//implement internal processes

var CMD_MAP = map[string]func(cmd* Cmd) Process {
    "execute": NewExtProcess,
}

func NewProcess(cmd *Cmd) Process {
    constructor, ok := CMD_MAP[cmd.name]
    if !ok {
        return nil
    }

    return constructor(cmd)
}
