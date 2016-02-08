package core

import (
	"encoding/json"
	"fmt"
	"github.com/Jumpscale/agent2/agent/lib/settings"
	"github.com/Jumpscale/agent2/agent/lib/utils"
	"log"
)

/*
MapArgs represents the run arguments of a command
*/
type MapArgs struct {
	tag        string
	controller *settings.Controller
	data       map[string]interface{}
}

/*
NewMapArgs creates a new MapArgs from a dict
*/
func NewMapArgs(data map[string]interface{}) *MapArgs {
	return &MapArgs{
		data: data,
	}
}

/*
MarshalJSON serializes to json
*/
func (args *MapArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(args.data)
}

/*
UnmarshalJSON unserializes from json
*/
func (args *MapArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &args.data)
}

/*
Data returns the MapArgs as a dict
*/
func (args *MapArgs) Data() map[string]interface{} {
	return args.data
}

func (args *MapArgs) ensureInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}

	return 0
}

func (args *MapArgs) ensureFloat(value interface{}) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	}

	return 0
}

//GetInt gets int value
func (args *MapArgs) GetInt(key string) int {
	s, ok := args.data[key]
	if !ok {
		return 0
	}

	return args.ensureInt(s)
}

//GetString gets string value
func (args *MapArgs) GetString(key string) string {
	s, ok := args.data[key]
	if ok {
		return fmt.Sprintf("%v", s)
	}
	return ""
}

func (args *MapArgs) GetStringDefault(key string, value string) string {
	s, ok := args.data[key]
	if ok {
		return fmt.Sprintf("%v", s)
	}
	return value
}

//GetFloat gets float value
func (args *MapArgs) GetFloat(key string) float64 {
	s, ok := args.data[key]
	if !ok {
		return 0
	}

	return args.ensureFloat(s)
}

//GetMap gets a dict value
func (args *MapArgs) GetMap(key string) map[string]interface{} {
	s, ok := args.data[key]
	if ok {
		return s.(map[string]interface{})
	}

	return make(map[string]interface{})
}

//GetArray gets array value
func (args *MapArgs) GetArray(key string) []interface{} {
	s, ok := args.data[key]
	if ok {
		return s.([]interface{})
	}

	return make([]interface{}, 0)
}

//GetStringArray gets an array of strings
func (args *MapArgs) GetStringArray(key string) []string {
	s, ok := args.data[key]
	if !ok {
		return []string{}
	}

	switch t := s.(type) {
	case []string:
		return t
	case []interface{}:
		values := make([]string, len(t))
		for i, v := range t {
			values[i] = fmt.Sprintf("%v", v)
		}
		args.data[key] = values
		return values
	}

	return []string{}
}

//GetIntArray gets array of ints
func (args *MapArgs) GetIntArray(key string) []int {
	s, ok := args.data[key]
	if !ok {
		return []int{}
	}

	switch t := s.(type) {
	case []int:
		return t
	case []interface{}:
		values := make([]int, len(t))
		for i, v := range t {
			values[i] = args.ensureInt(v)
		}
		args.data[key] = values
		return values
	case string:
		//requires expansion.
		values, err := utils.Expand(t)
		if err != nil {
			log.Println("Invalid array string", t)
			return []int{}
		}
		args.data[key] = values
		return values
	}

	return []int{}
}

//Set overrides a value in the ArgsMap
func (args *MapArgs) Set(key string, value interface{}) {
	args.data[key] = value
}

//Clone returns a cloned instance of the MapArgs
func (args *MapArgs) Clone(deep bool) *MapArgs {
	data := make(map[string]interface{})
	for k, v := range args.data {
		if deep {
			switch tv := v.(type) {
			case []int:
				l := make([]int, len(tv))
				copy(l, tv)
				data[k] = l
			case []string:
				l := make([]string, len(tv))
				copy(l, tv)
				data[k] = l
			}
		} else {
			data[k] = v
		}
	}

	return &MapArgs{
		tag:        args.tag,
		controller: args.controller,
		data:       data,
	}
}

//SetTag stores a string tag on the object (not serialized)
func (args *MapArgs) SetTag(tag string) {
	args.tag = tag
}

//GetTag gets the tag value
func (args *MapArgs) GetTag() string {
	return args.tag
}

//SetController stores the controller info of the associated controller (not serialized)
func (args *MapArgs) SetController(controller *settings.Controller) {
	args.controller = controller
}

//GetController gets the associated controller configuration
func (args *MapArgs) GetController() *settings.Controller {
	return args.controller
}
