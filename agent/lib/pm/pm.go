package pm

import (
    "io/ioutil"
    "time"
    "log"
    "fmt"
    "sync"
    "strconv"
    "github.com/Jumpscale/jsagent/agent/lib/utils"
    "github.com/shirou/gopsutil/process"
)

var RESULT_MESSAGE_LEVELS []int = []int{20, 21, 22, 23, 30}

type Cmd struct {
    name string
    id string
    args Args
    data string
}


type MeterHandler func (cmd *Cmd, p *process.Process)
type MessageHandler func (msg *Message)
type ResultHandler func(result *JobResult)


type PM struct {
    mid uint32
    midfile string
    midMux *sync.Mutex
    cmds chan *Cmd
    processes map[string]Process
    meterHandlers []MeterHandler
    msgHandlers []MessageHandler
    resultHandlers []ResultHandler
}


func NewPM(midfile string) *PM {
    pm := &PM{
        cmds: make(chan *Cmd),
        midfile: midfile,
        mid: loadMid(midfile),
        midMux: &sync.Mutex{},
        processes: make(map[string]Process),
        meterHandlers: make([]MeterHandler, 0, 3),
        msgHandlers: make([]MessageHandler, 0, 3),
        resultHandlers: make([]ResultHandler, 0, 3),
    }
    return pm
}

func loadMid(midfile string) uint32 {
    content, err := ioutil.ReadFile(midfile)
    if err != nil {
        log.Println(err)
        return 0
    }
    v, err := strconv.ParseUint(string(content), 10, 32)
    if err != nil {
        log.Println(err)
        return 0
    }
    return uint32(v)
}

func saveMid(midfile string, mid uint32) {
    ioutil.WriteFile(midfile, []byte(fmt.Sprintf("%d", mid)), 0644)
}

func (pm *PM) NewCmd(name string, id string, args Args, data string) {
    cmd := &Cmd {
        id: id,
        name: name,
        args: args,
        data: data,
    }

    pm.cmds <- cmd
}

func (pm *PM) getNextMsgID() uint32 {
    pm.midMux.Lock()
    defer pm.midMux.Unlock()
    pm.mid += 1
    saveMid(pm.midfile, pm.mid)
    return pm.mid
}

func (pm *PM) AddMeterHandler(handler MeterHandler) {
    pm.meterHandlers = append(pm.meterHandlers, handler)
}

func (pm *PM) AddMessageHandler(handler MessageHandler) {
    pm.msgHandlers = append(pm.msgHandlers, handler)
}

func (pm *PM) AddResultHandler(handler ResultHandler) {
    pm.resultHandlers = append(pm.resultHandlers, handler)
}

func (pm *PM) Run() {
    //process and start all commands according to args.
    go func() {
        for {
            cmd := <- pm.cmds
            process := NewExtProcess(cmd)

            pm.processes[cmd.id] = process // do we really need this ?
            go process.run(runCfg{
                meterHandler: pm.meterCallback,
                msgHandler: pm.msgCallback,
                resultHandler: pm.resultCallback,
            })
        }
    }()
}

func (pm *PM) meterCallback(cmd *Cmd, ps *process.Process) {
    for _, handler := range pm.meterHandlers {
        handler(cmd, ps)
    }
}

func (pm *PM) msgCallback(msg *Message) {
    if !utils.In(msg.cmd.args.GetLogLevels(), msg.level) {
        return
    }

    //stamp msg.
    msg.epoch = time.Now().Unix()
    //add ID
    msg.id = pm.getNextMsgID()
    for _, handler := range pm.msgHandlers {
        handler(msg)
    }
}

func (pm *PM) resultCallback(result *JobResult) {
    for _, handler := range pm.resultHandlers {
        handler(result)
    }
}
