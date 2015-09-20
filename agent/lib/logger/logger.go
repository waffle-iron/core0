package logger

import (
	"bytes"
	"encoding/json"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"log"
	"net/http"
	"time"
)

type Logger interface {
	Log(msg *pm.Message)
}

type DBLogger struct {
	factory  DBFactory
	defaults []int
}

//Creates a new Database logger, it stores the logged message in database
//factory: is the DB connection factory
//defaults: default log levels to store in db if is not specificed by the logged message.
func NewDBLogger(factory DBFactory, defaults []int) Logger {
	return &DBLogger{
		factory:  factory,
		defaults: defaults,
	}
}

//Log message
func (logger *DBLogger) Log(msg *pm.Message) {
	levels := logger.defaults
	msgLevels := msg.Cmd.Args.GetIntArray("loglevels_db")

	if len(msgLevels) > 0 {
		levels = msgLevels
	}

	if len(levels) > 0 && !utils.In(levels, msg.Level) {
		return
	}

	db := logger.factory.GetDBCon()
	stmnt := `
        insert into logs (id, jobid, domain, name, epoch, level, data)
        values (?, ?, ?, ?, ?, ?, ?)
    `
	args := msg.Cmd.Args
	_, err := db.Exec(stmnt, msg.Id, msg.Cmd.Id, args.GetString("domain"), args.GetString("name"),
		msg.Epoch, msg.Level, msg.Message)
	if err != nil {
		log.Println(err)
	}
}

type ACLogger struct {
	endpoints map[string]*http.Client
	buffer    utils.Buffer
	defaults  []int
}

//Create a new AC logger. AC logger buffers log messages into bulks and batch send it to the given end points over HTTP (POST)
//endpoints: list of URLs that the AC logger will post the batches to
//bufsize: Max number of messages to keep before sending the data to the end points
//flushInt: Max time to wait before sending data to the end points. So either a full buffer or flushInt can force flushing
//   the messages
//defaults: default log levels to store in db if is not specificed by the logged message.
func NewACLogger(endpoints map[string]*http.Client, bufsize int, flushInt time.Duration, defaults []int) Logger {
	logger := &ACLogger{
		endpoints: endpoints,
		defaults:  defaults,
	}

	logger.buffer = utils.NewBuffer(bufsize, flushInt, logger.send)

	return logger
}

//Log message
func (logger *ACLogger) Log(msg *pm.Message) {
	levels := logger.defaults
	msgLevels := msg.Cmd.Args.GetIntArray("loglevels_db")

	if len(msgLevels) > 0 {
		levels = msgLevels
	}

	if len(levels) > 0 && !utils.In(levels, msg.Level) {
		return
	}

	logger.buffer.Append(msg)
}

func (logger *ACLogger) send(objs []interface{}) {
	if len(objs) == 0 {
		//objs can be of length zero, when flushed on timer while
		//no messages are ready.
		return
	}

	msgs, err := json.Marshal(objs)
	if err != nil {
		log.Println("Failed to serialize the logs")
		return
	}

	reader := bytes.NewReader(msgs)
	for endpoint, client := range logger.endpoints {
		resp, err := client.Post(endpoint, "application/json", reader)
		if err != nil {
			log.Println("Failed to send log batch to AC", endpoint, err)
			continue
		}
		defer resp.Body.Close()
	}
}

type ConsoleLogger struct {
	defaults []int
}

//Simple console logger that prints log messages to Console.
func NewConsoleLogger(defaults []int) Logger {
	return &ConsoleLogger{
		defaults: defaults,
	}
}

func (logger *ConsoleLogger) Log(msg *pm.Message) {
	if len(logger.defaults) > 0 && !utils.In(logger.defaults, msg.Level) {
		return
	}

	log.Println(msg)
}
