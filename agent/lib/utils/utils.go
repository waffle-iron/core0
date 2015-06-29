package utils

import (
    "github.com/naoina/toml"
    "io/ioutil"
    "os"
)
func Expand(litral string, min int, max int) []int {
    return []int{1, 2, 3}
}

//Checks if x is in l
func In(l []int, x int) bool {
    for i := 0; i < len(l); i++ {
        if l[i] == x {
            return true
        }
    }

    return false
}

func Update(dst map[string]interface{}, src map[string]interface{}){
    for k, v := range src {
        dst[k] = v
    }
}

//LoadTomlFile loads toml using "github.com/naoina/toml"
func LoadTomlFile(filename string, v interface{}) {
    f, err := os.Open(filename)
    if err != nil {
        panic(err)
    }
    defer f.Close()
    buf, err := ioutil.ReadAll(f)
    if err != nil {
        panic(err)
    }
    if err := toml.Unmarshal(buf, v); err != nil {
        panic(err)
    }
}
