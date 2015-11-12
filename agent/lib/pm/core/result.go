package core

//JobResult represents a result of a job
type JobResult struct {
	ID        string   `json:"id"`
	Gid       int      `json:"gid"`
	Nid       int      `json:"nid"`
	Cmd       string   `json:"cmd"`
	Args      *MapArgs `json:"args"`
	Data      string   `json:"data"`
	Streams   []string `json:"streams,omitempty"`
	Critical  string   `json:"critical,omitempty"`
	Level     int      `json:"level"`
	State     string   `json:"state"`
	StartTime int64    `json:"starttime"`
	Time      int64    `json:"time"`
	Tags      string   `json:"tags"`
}
