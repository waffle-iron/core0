package logger

import (
	"database/sql"
	"fmt"
	"github.com/Jumpscale/jsagent/agent/lib/utils"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"log"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const (
	MAX_RESULT_COUNT = 1000
)

var FILE_REGEXP *regexp.Regexp = regexp.MustCompile("^(\\d+)\\.db$")

type MsgQuery interface {
	Query(Query) (<-chan Result, error)
}

type dbMsgQuery struct {
	path string
}

func NewDBMsgQuery(path string) MsgQuery {
	return &dbMsgQuery{
		path: path,
	}
}

type Query struct {
	JobID    string      `json:"jobid"`
	TimeFrom int64       `json:"timefrom"`
	TimeTo   int64       `json:"timeto"`
	Levels   interface{} `json:"levels"`
	Limit    int         `json:"limit"`
}

type Result struct {
	Id     int    `json:"id"`
	JobID  string `json:"jobid"`
	Domain string `json:"domain"`
	Name   string `json:"name"`
	Epoch  int    `json:"epoch"`
	Level  int    `json:"level"`
	Data   string `json:"data"`
}

func (self *dbMsgQuery) Query(query Query) (<-chan Result, error) {
	filesinfos, err := ioutil.ReadDir(self.path)
	if err != nil {
		return nil, err
	}

	limit := MAX_RESULT_COUNT
	if query.Limit > 0 && query.Limit < MAX_RESULT_COUNT {
		limit = query.Limit
	}

	files := make([]string, 0, 10)

	lower_added := false
	for _, fileinfo := range filesinfos {
		if fileinfo.IsDir() {
			continue
		}

		matches := FILE_REGEXP.FindStringSubmatch(fileinfo.Name())
		if len(matches) > 0 {
			stamp, _ := strconv.ParseInt(matches[1], 10, 64)
			if query.TimeFrom > 0 && query.TimeFrom > stamp {
				// older than time-from
				continue
			}

			if query.TimeTo > 0 && query.TimeTo < stamp {
				// we must add the fist file with stamp > TimeTo because
				// it still considered in range
				if lower_added {
					continue
				}
				lower_added = true
			}
			//TODO add more logic to better detect files for queries.
			files = append(files, fileinfo.Name())
		}
	}

	if !lower_added {
		files = append(files, "current.db")
	}

	var levels []int

	//loading levels.
	if query.Levels != nil {
		switch ls := query.Levels.(type) {
		case string:
			levels, err = utils.Expand(ls)
			if err != nil {
				return nil, err
			}
		case []int:
			levels = ls
		case []float64:
			//happens when unmarshaling from json
			levels = make([]int, len(ls))
			for i := 0; i < len(ls); i++ {
				levels[i] = int(ls[i])
			}
		}
	} else {
		levels = make([]int, 0)
	}

	where := make([]string, 0)
	params := make([]interface{}, 0)

	if query.JobID != "" {
		where = append(where, "jobid = ?")
		params = append(params, query.JobID)
	}

	if query.TimeFrom > 0 {
		where = append(where, "epoch >= ?")
		params = append(params, query.TimeFrom)
	}

	if query.TimeTo > 0 {
		where = append(where, "epoch <= ?")
		params = append(params, query.TimeTo)
	}

	if len(levels) > 0 {

		levels_str := make([]string, len(levels))
		for i, l := range levels {
			levels_str[i] = strconv.Itoa(l)
		}

		expr := fmt.Sprintf("level in (%s)", strings.Join(levels_str, ","))
		where = append(where, expr)
	}

	count := 0
	results := make(chan Result)
	// search the filtered files.
	go func() {
	allquery:
		for t := len(files) - 1; t >= 0; t-- {
			dbfile := files[t]
			log.Println("Query:", dbfile)
			dbpath := path.Join(self.path, dbfile)

			db, err := sql.Open("sqlite3", dbpath)
			if err != nil {
				//couldn't open db file for reading
				//let's just continue to the next one for now
				log.Println(err)
				continue
			}

			defer db.Close()

			query := "select id, jobid, domain, name, epoch, level, data from logs"
			if len(where) > 0 {
				query += " where " + strings.Join(where, " and ")
			}

			query += fmt.Sprintf(" order by id desc limit %d;", limit)

			rows, err := db.Query(query, params...)

			if err != nil {
				log.Println(err)
				continue
			}

			defer rows.Close()

			var id int
			var jobid string
			var domain string
			var name string
			var epoch int
			var level int
			var data string

			for rows.Next() {
				if err := rows.Scan(&id, &jobid, &domain, &name, &epoch, &level, &data); err != nil {
					//couldn't read this row!! ignore and move on for now
					log.Println(err)
					continue
				}

				results <- Result{
					Id:     id,
					JobID:  jobid,
					Domain: domain,
					Name:   name,
					Epoch:  epoch,
					Level:  level,
					Data:   data,
				}

				count += 1
				if count >= limit {
					break allquery
				}
			}
		}

		close(results)
	}()

	return results, nil
}
