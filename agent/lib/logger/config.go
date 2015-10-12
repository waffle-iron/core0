package logger

import (
	"github.com/Jumpscale/agent2/agent"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/boltdb/bolt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

func ConfigureLogging(mgr *pm.PM, controllers map[string]*agent.ControllerClient, settings *agent.Settings) {
	//apply logging handlers.
	dbLoggerConfigured := false
	for _, logcfg := range settings.Logging {
		switch strings.ToLower(logcfg.Type) {
		case "db":
			if dbLoggerConfigured {
				log.Fatal("Only one db logger can be configured")
			}
			//sqlFactory := logger.NewSqliteFactory(logcfg.LogDir)
			os.Mkdir(logcfg.LogDir, 0755)
			db, err := bolt.Open(path.Join(logcfg.LogDir, "logs.db"), 0644, nil)
			db.MaxBatchDelay = 100 * time.Millisecond
			if err != nil {
				log.Fatal("Failed to open logs database", err)
			}

			handler, err := NewDBLogger(db, logcfg.Levels)
			if err != nil {
				log.Fatal("DB logger failed to initialize", err)
			}
			mgr.AddMessageHandler(handler.Log)
			registerGetMsgsFunction(db)

			dbLoggerConfigured = true
		case "ac":
			endpoints := make(map[string]*http.Client)

			if len(logcfg.Controllers) > 0 {
				//specific ones.
				for _, key := range logcfg.Controllers {
					controller, ok := controllers[key]
					if !ok {
						log.Fatalf("Unknow controller '%s'", key)
					}
					url := controller.BuildUrl(settings.Main.Gid, settings.Main.Nid, "log")
					endpoints[url] = controller.Client
				}
			} else {
				//all ACs
				for _, controller := range controllers {
					url := controller.BuildUrl(settings.Main.Gid, settings.Main.Nid, "log")
					endpoints[url] = controller.Client
				}
			}

			batchsize := 1000 // default
			flushint := 120   // default (in seconds)
			if logcfg.BatchSize != 0 {
				batchsize = logcfg.BatchSize
			}
			if logcfg.FlushInt != 0 {
				flushint = logcfg.FlushInt
			}

			handler := NewACLogger(
				endpoints,
				batchsize,
				time.Duration(flushint)*time.Second,
				logcfg.Levels)
			mgr.AddMessageHandler(handler.Log)
		case "console":
			handler := NewConsoleLogger(logcfg.Levels)
			mgr.AddMessageHandler(handler.Log)
		default:
			log.Fatalf("Unsupported logger type: %s", logcfg.Type)
		}
	}
}
