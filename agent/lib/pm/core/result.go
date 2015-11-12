package core

const (
	//StateSuccess successs exit status
	StateSuccess = "SUCCESS"
	//StateError error exist status
	StateError = "ERROR"
	//StateTimeout timeout exit status
	StateTimeout = "TIMEOUT"
	//StateKilled killed exit status
	StateKilled = "KILLED"
	//StateUnknownCmd unknown cmd exit status
	StateUnknownCmd = "UNKNOWN_CMD"
	//StateDuplicateID dublicate id exit status
	StateDuplicateID = "DUPILICATE_ID"
)

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

//NewBasicJobResult creates a new job result from command
func NewBasicJobResult(cmd *Cmd) *JobResult {
	return &JobResult{
		ID:   cmd.ID,
		Gid:  cmd.Gid,
		Nid:  cmd.Nid,
		Cmd:  cmd.Name,
		Args: cmd.Args,
	}
}
