package pm

//implement internal processes

import (
    "os"
    "time"
    "github.com/shirou/gopsutil/process"

    // "database/sql"
    // _ "github.com/mattn/go-sqlite3"
)

const (
    CMD_EXECUTE = "execute"
    CMD_EXECUTE_JS_PY = "execute_js_py"
    CMD_EXECUTE_JS_LUA = "execute_js_lua"
    CMD_GET_MSGS = "get_msgs"
    CMD_PING = "ping"
    CMD_RESTART = "restart"
    CMD_KILLALL = "killall"
)

type ProcessConstructor func (cmd *Cmd) Process

var CMD_MAP = map[string]ProcessConstructor {
    CMD_EXECUTE: NewExtProcess,
    CMD_EXECUTE_JS_PY: extScript("python2.7", "./python", ""),
    CMD_EXECUTE_JS_LUA: extScript("lua", "./lua", ""),
    // CMD_GET_MSGS: extScript("python2.7", "./builtin", "get_msgs.py"),
    CMD_PING: internalScript(ping),
    CMD_RESTART: internalScript(restart),
    CMD_KILLALL: internalScript(killall),
}


func NewProcess(cmd *Cmd) Process {
    constructor, ok := CMD_MAP[cmd.name]
    if !ok {
        return nil
    }

    return constructor(cmd)
}


type JsScriptProcess struct {
    extps Process
    cmd *Cmd
}

//Create a constructor for external process to execute an external script
//exe: name of executor, (python, lua, bash)
//workdir: working directory of script
//name: if name != "", execute that specific script, otherwise use args[name]
func extScript(exe string, workdir string, name string) ProcessConstructor {
    //create a new execute process with python2.7 or lua as executors.
    constructor := func(cmd *Cmd) Process {
        args := cmd.args.Clone(false)
        var script string

        if name != "" {
            script = name
        } else {
            script = cmd.args.GetString("name")
        }

        args.Set("name", exe)
        args.Set("args", []string{script})
        args.Set("working_dir", workdir)

        extcmd := &Cmd {
            id: cmd.id,
            gid: cmd.gid,
            nid: cmd.nid,
            name: CMD_EXECUTE,
            data: cmd.data,
            args: args,
        }

        return &JsScriptProcess{
            extps: NewExtProcess(extcmd),
            cmd: cmd,
        }
    }

    return constructor
}


func (ps *JsScriptProcess) run(cfg RunCfg) {
    //intercept all the messages from the 'execute' command and
    //change it to it's original value.
    extcfg := RunCfg{
        ProcessManager: cfg.ProcessManager,
        MeterHandler: func(cmd *Cmd, p *process.Process) {
            cfg.MeterHandler(ps.cmd, p)
            },
        MessageHandler: func(msg *Message) {
            msg.Cmd = ps.cmd
            cfg.MessageHandler(msg)
            },
        ResultHandler: func(result *JobResult) {
            result.Args = ps.cmd.args
            result.Cmd = ps.cmd.name
            cfg.ResultHandler(result)
            },
        Signal: cfg.Signal,
    }

    ps.extps.run(extcfg)
}

func (ps *JsScriptProcess) kill() {
    ps.extps.kill()
}

type Runable func (*Cmd, RunCfg)

type internalProcess struct {
    runable Runable
    cmd *Cmd
}

func internalScript (runable Runable) ProcessConstructor {
    constructor := func(cmd *Cmd) Process {
        return &internalProcess{
            runable: runable,
            cmd: cmd,
        }
    }

    return constructor
}

func (ps *internalProcess) run(cfg RunCfg) {
    defer func() {
        cfg.Signal <- 1
    }()

    ps.runable(ps.cmd, cfg)
}

func (ps *internalProcess) kill (){
    //you can't kill an internal process.
}

func ping(cmd *Cmd, cfg RunCfg) {
    result := &JobResult {
        Id: cmd.id,
        Gid: cmd.gid,
        Nid: cmd.nid,
        Args: cmd.args,
        StartTime: time.Now().Unix(),
        Time: 0,
        State: "SUCCESS",
        Level: L_RESULT_JSON,
        Data: `"pong"`,
    }

    cfg.ResultHandler(result)
}


func restart(cmd *Cmd, cfg RunCfg) {
    os.Exit(0)
}

func killall(cmd *Cmd, cfg RunCfg) {
    cfg.ProcessManager.Killall()
}

// func getProcessStats(cmd *Cmd, cfg RunCfg) {
//     for _, process := range cfg.ProcessManager.processes {
//         //only work with external processes
//         switch p := process.(type) {
//         case ExtProcess:
//             pid := p.pid
//         }
//     }
// }
