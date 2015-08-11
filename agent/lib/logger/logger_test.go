package logger

import (
	"encoding/json"
	"github.com/Jumpscale/jsagent/agent/lib/pm"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

func getFakeCmd(t *testing.T) *pm.Cmd {
	cmd, err := pm.LoadCmd([]byte("{}"))
	if err != nil {
		t.Error("Could not create fake command")
	}
	return cmd
}

func TestDBLogger_Basic(t *testing.T) {
	testdb := os.TempDir() + "/testdb"

	factory := NewSqliteFactory(testdb)
	defer os.RemoveAll(testdb)

	logger := NewDBLogger(factory, []int{1, 2})

	message := "Hello world"

	msg1 := &pm.Message{
		Id:      1,
		Cmd:     getFakeCmd(t),
		Level:   1,
		Message: message,
		Epoch:   1000,
	}

	logger.Log(msg1)
	db := factory.GetDBCon()

	rows, err := db.Query("select level, data, epoch from logs limit 10;")
	if err != nil {
		t.Error(err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count += 1
		var level int
		var data string
		var epoch int
		if err := rows.Scan(&level, &data, &epoch); err != nil {
			t.Error("Couldn't load stored data")
		}
		if data != message {
			t.Error("Wrong log message found")
		}
	}

	if count != 1 {
		t.Error("Wrong number of records in DB", count)
	}
}

func TestDBLogger_LevelsFilter(t *testing.T) {
	testdb := os.TempDir() + "/testdb"

	factory := NewSqliteFactory(testdb)
	defer os.RemoveAll(testdb)

	logger := NewDBLogger(factory, []int{1, 2})

	message1 := "Hello world"
	message2 := "Bye Bye World"

	msg1 := &pm.Message{
		Id:      1,
		Cmd:     getFakeCmd(t),
		Level:   1,
		Message: message1,
		Epoch:   1000,
	}

	msg2 := &pm.Message{
		Id:      1,
		Cmd:     getFakeCmd(t),
		Level:   4,
		Message: message2,
		Epoch:   1000,
	}

	//only one message will get logged, since msg2 has wrong level
	logger.Log(msg1)
	logger.Log(msg2)

	db := factory.GetDBCon()

	rows, err := db.Query("select level, data, epoch from logs limit 10;")
	if err != nil {
		t.Error(err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count += 1
		var level int
		var data string
		var epoch int
		if err := rows.Scan(&level, &data, &epoch); err != nil {
			t.Error("Couldn't load stored data")
		}
		if data != message1 {
			t.Error("Wrong log message found")
		}
	}

	if count != 1 {
		t.Error("Wrong number of records in DB", count)
	}
}

func TestDBLogger_ForceLevel(t *testing.T) {
	testdb := os.TempDir() + "/testdb"

	factory := NewSqliteFactory(testdb)
	defer os.RemoveAll(testdb)

	logger := NewDBLogger(factory, []int{1, 2})

	message1 := "Hello world"

	msg := &pm.Message{
		Id:      1,
		Cmd:     getFakeCmd(t),
		Level:   4,
		Message: message1,
		Epoch:   1000,
	}

	//override cmd args
	msg.Cmd.Args.Set("loglevels_db", []int{4})

	//only one message will get logged, since msg2 has wrong level
	logger.Log(msg)

	db := factory.GetDBCon()

	rows, err := db.Query("select level, data, epoch from logs limit 10;")
	if err != nil {
		t.Error(err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count += 1
		var level int
		var data string
		var epoch int
		if err := rows.Scan(&level, &data, &epoch); err != nil {
			t.Error("Couldn't load stored data")
		}
		if data != message1 {
			t.Error("Wrong log message found")
		}
	}

	if count != 1 {
		t.Error("Wrong number of records in DB", count)
	}
}

func TestACLogger_BatchSizeTrigger(t *testing.T) {
	var wg sync.WaitGroup

	serverMux := http.NewServeMux()
	server := &http.Server{
		Handler: serverMux,
	}

	listner, err := net.Listen("tcp", ":1234")
	if err != nil {
		t.Error(err)
	}

	signal := make(chan int)
	handle := func(writer http.ResponseWriter, request *http.Request) {
		defer listner.Close()
		defer request.Body.Close()
		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
		}

		//content is the serialized log messages.
		var messages []*pm.Message
		json.Unmarshal(body, &messages)

		signal <- len(messages)
	}

	wg.Add(1)
	//starting proxy
	go func() {
		defer wg.Done()
		serverMux.HandleFunc("/logs", handle)
		go server.Serve(listner)
	}()

	//wait until proxy is ready before starting agents.
	wg.Wait()

	logger := NewACLogger(map[string]*http.Client{
		"http://localhost:1234/logs": &http.Client{},
	}, 2, 60*time.Minute, []int{1, 2})

	message1 := "Hello world"

	msg1 := &pm.Message{
		Id:      1,
		Cmd:     getFakeCmd(t),
		Level:   1,
		Message: message1,
		Epoch:   1000,
	}

	msg2 := &pm.Message{
		Id:      2,
		Cmd:     getFakeCmd(t),
		Level:   1,
		Message: message1,
		Epoch:   1000,
	}

	logger.Log(msg1)
	logger.Log(msg2)

	select {
	case l := <-signal:
		if l != 2 {
			t.Error("Invalid number of messages logged")
		}
	case <-time.After(10 * time.Second):
		t.Error("Something went wrong, messages never received at the end point")
	}
}

func TestACLogger_FlushIntTrigger(t *testing.T) {
	var wg sync.WaitGroup

	serverMux := http.NewServeMux()
	server := &http.Server{
		Handler: serverMux,
	}

	listner, err := net.Listen("tcp", ":1236")
	if err != nil {
		t.Error(err)
	}

	signal := make(chan int)
	handle := func(writer http.ResponseWriter, request *http.Request) {
		defer listner.Close()
		defer request.Body.Close()
		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
		}

		//content is the serialized log messages.
		var messages []*pm.Message
		json.Unmarshal(body, &messages)

		signal <- len(messages)
	}

	wg.Add(1)
	//starting proxy
	go func() {
		defer wg.Done()
		serverMux.HandleFunc("/logs", handle)
		go server.Serve(listner)
	}()

	//wait until proxy is ready before starting agents.
	wg.Wait()

	logger := NewACLogger(map[string]*http.Client{
		"http://localhost:1236/logs": &http.Client{},
	}, 100, 5*time.Second, []int{1, 2})

	message1 := "Hello world"

	msg1 := &pm.Message{
		Id:      1,
		Cmd:     getFakeCmd(t),
		Level:   1,
		Message: message1,
		Epoch:   1000,
	}

	msg2 := &pm.Message{
		Id:      2,
		Cmd:     getFakeCmd(t),
		Level:   1,
		Message: message1,
		Epoch:   1000,
	}

	msg3 := &pm.Message{
		Id:      2,
		Cmd:     getFakeCmd(t),
		Level:   5,
		Message: message1,
		Epoch:   1000,
	}

	logger.Log(msg1)
	logger.Log(msg2)
	logger.Log(msg3)

	select {
	case l := <-signal:
		if l != 2 {
			t.Error("Invalid number of messages logged")
		}
	case <-time.After(10 * time.Second):
		t.Error("Something went wrong, messages never received at the end point")
	}
}
