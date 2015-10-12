package logger

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/builtin"
	"github.com/Jumpscale/agent2/agent/lib/pm"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"github.com/boltdb/bolt"
	"log"
	"time"
)

type LogQuery struct {
	JobID  string      `json:"jobid"`
	Levels interface{} `json:"levels"`
	Limit  int         `json:"limit"`
}

const (
	CmdGetMsgs             = "get_msgs"
	CmdGetMsgsDefaultLimit = 1000
)

func registerGetMsgsFunction(db *bolt.DB) {

	get_levels := func(levels interface{}) ([]int, error) {
		var results []int

		if levels == nil {
			levels = "*"
		}

		//loading levels.
		if levels != nil {
			switch ls := levels.(type) {
			case string:
				var err error
				results, err = utils.Expand(ls)
				if err != nil {
					return nil, err
				}
			case []int:
				results = ls
			case []float64:
				//happens when unmarshaling from json
				results = make([]int, len(ls))
				for i := 0; i < len(ls); i++ {
					results[i] = int(ls[i])
				}
			}
		} else {
			levels = make([]int, 0)
		}

		return results, nil
	}

	get_msgs := func(cmd *pm.Cmd, cfg pm.RunCfg) *pm.JobResult {
		result := pm.NewBasicJobResult(cmd)
		result.StartTime = int64(time.Duration(time.Now().UnixNano()) / time.Millisecond)

		defer func() {
			endtime := time.Duration(time.Now().UnixNano()) / time.Millisecond
			result.Time = int64(endtime) - result.StartTime
		}()

		query := LogQuery{}

		err := json.Unmarshal([]byte(cmd.Data), &query)
		if err != nil {
			result.State = pm.S_ERROR
			result.Data = fmt.Sprintf("Failed to parse get_msgs query: %s", err)

			return result
		}

		if query.JobID == "" {
			result.State = pm.S_ERROR
			result.Data = "jobid is required"

			return result
		}

		levels, err := get_levels(query.Levels)
		if err != nil {
			result.State = pm.S_ERROR
			result.Data = fmt.Sprintf("Error parsing levels (%s): %s", query.Levels, err)

			return result
		}

		var limit int
		if query.Limit != 0 {
			limit = query.Limit
		}

		if limit > CmdGetMsgsDefaultLimit {
			limit = CmdGetMsgsDefaultLimit
		}

		//we still can continue the query even if we have unmarshal errors.
		records := make([]map[string]interface{}, 0, CmdGetMsgsDefaultLimit)

		err = db.View(func(tx *bolt.Tx) error {
			logs := tx.Bucket([]byte("logs"))
			if logs == nil {
				return errors.New("Logs database is not initialized")
			}

			job := logs.Bucket([]byte(query.JobID))
			if job == nil {
				log.Println("Failed to open job bucket")
				return nil
			}
			cursor := job.Cursor()
			for key, value := cursor.Last(); key != nil && len(records) < limit; key, value = cursor.Prev() {
				row := make(map[string]interface{})
				err := json.Unmarshal(value, &row)
				if err != nil {
					log.Printf("Failed to load job log '%s'\n", value)
					return err
				}
				if utils.In(levels, int(row["level"].(float64))) {
					records = append(records, row)
				}
			}
			return nil
		})

		if err != nil {
			result.State = pm.S_ERROR
			result.Data = fmt.Sprintf("%v", err)

			return result
		}

		data, err := json.Marshal(records)
		if err != nil {
			result.State = pm.S_ERROR
			result.Data = fmt.Sprintf("%v", err)

			return result
		}

		result.State = pm.S_SUCCESS
		result.Level = pm.L_RESULT_JSON
		result.Data = string(data)

		return result
	}

	pm.CMD_MAP[CmdGetMsgs] = builtin.InternalProcessFactory(get_msgs)
}
