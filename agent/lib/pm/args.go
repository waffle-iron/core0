package pm

import (
    "encoding/json"
)

type Args interface {
    GetInt(key string) int
    GetString(key string) string
    GetFloat(key string) float64
    GetMap(key string) map[string]interface{}
    GetArray(key string) []interface{}
    GetStringArray(key string) []string
    GetIntArray(key string) []int
    Set(key string, value interface{})
    SetTag(tag int)
    GetTag() int
    Clone(deep bool) Args
}

type MapArgs struct {
    tag int
    data map[string]interface{}
}

func NewMapArgs(data map[string]interface{}) Args{
    return &MapArgs{
        data: data,
    }
}

func (args *MapArgs) MarshalJSON() ([]byte, error) {
    return json.Marshal(args.data)
}

func (args *MapArgs) UnmarshalJSON(data []byte) error {
    return json.Unmarshal(data, &args.data)
}

func (args *MapArgs) ensureInt(value interface{}) int {
    switch v := value.(type) {
    case int:
        return v
    case float64:
        return int(v)
    }

    return 0
}

func (args *MapArgs) GetInt(key string) int {
    s, ok := args.data[key]
    if !ok {
        return 0
    }

    return args.ensureInt(s)
}

func (args *MapArgs) GetString(key string) string {
    s, ok := args.data[key]
    if ok {
        return s.(string)
    }
    return ""
}

func (args *MapArgs) GetFloat(key string) float64 {
    s, ok := args.data[key]
    if ok {
        return s.(float64)
    }
    return 0
}

func (args *MapArgs) GetMap(key string) map[string]interface{} {
    s, ok := args.data[key]
    if ok {
        return s.(map[string]interface{})
    }

    return make(map[string]interface{})
}

func (args *MapArgs) GetArray(key string) []interface{} {
    s, ok := args.data[key]
    if ok {
        return s.([]interface{})
    }

    return make([]interface{}, 0)
}

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
            values[i] = v.(string)
        }
        return values
    }

    return []string{}
}

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
        return values
    }

    return []int{}
}

func (args *MapArgs) Set(key string, value interface{}) {
    args.data[key] = value
}

func (args *MapArgs) Clone(deep bool) Args {
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
        tag: args.tag,
        data: data,
    }
}

func (args *MapArgs) SetTag(tag int) {
    args.tag = tag
}

func (args *MapArgs) GetTag() int {
    return args.tag
}
