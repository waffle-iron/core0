package utils

import (
	"errors"
	"fmt"
	"github.com/Jumpscale/agent2/agent"
	"github.com/naoina/toml"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	CONFIG_SUFFIX = ".toml"
)

var valid_levels []int = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 20, 21, 22, 23, 30}

func Expand(s string) ([]int, error) {
	levels := make(map[int]bool)

	for _, part := range strings.Split(s, ",") {
		part := strings.Trim(part, " ")

		boundries := strings.Split(part, "-")
		lower := strings.Trim(boundries[0], " ")
		lower_value := 0
		upper_value := 0
		has_upper := false

		if len(boundries) > 1 {

			upper_value_64, err := strconv.ParseInt(strings.Trim(boundries[1], " "), 10, 32)
			if err != nil {
				return nil, err
			}

			has_upper = true
			upper_value = int(upper_value_64)
		}

		if lower == "*" {
			for _, l := range valid_levels {
				levels[l] = true
			}
			continue
		}

		lower_value_64, err := strconv.ParseInt(lower, 10, 32)
		if err != nil {
			return nil, err
		}

		lower_value = int(lower_value_64)

		if !has_upper {
			if In(valid_levels, lower_value) {
				levels[lower_value] = true
			}

			continue
		}

		if upper_value > 30 {
			upper_value = 30
		}

		for i := lower_value; i <= upper_value; i++ {
			if In(valid_levels, i) {
				levels[i] = true
			}
		}
	}

	result := make([]int, 0, len(levels))
	for key, _ := range levels {
		result = append(result, key)
	}

	sort.Ints(result)
	return result, nil
}

var formatPattern *regexp.Regexp = regexp.MustCompile("{[^}]+}")

func Format(pattern string, values map[string]interface{}) string {
	return formatPattern.ReplaceAllStringFunc(pattern, func(m string) string {
		key := strings.TrimRight(strings.TrimLeft(m, "{"), "}")
		return fmt.Sprintf("%v", values[key])
	})
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

func Update(dst map[string]interface{}, src map[string]interface{}) {
	for k, v := range src {
		dst[k] = v
	}
}

//LoadTomlFile loads toml using "github.com/naoina/toml"
func LoadTomlFile(filename string, v interface{}) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	if err := toml.Unmarshal(buf, v); err != nil {
		return err
	}

	return nil
}

func GetPartialSettings(settings *agent.Settings) (*agent.PartialSettings, error) {
	partial := &agent.PartialSettings{
		Extensions: make(map[string]agent.Extension),
		Startup:    make(map[string]agent.StartupCmd),
	}

	if settings.Main.Include == "" {
		return partial, nil
	}

	infos, err := ioutil.ReadDir(settings.Main.Include)
	if err != nil {
		return nil, err
	}

	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		name := info.Name()
		if len(name) <= len(CONFIG_SUFFIX) {
			//file name too short to be a config file (shorter than the extension)
			continue
		}
		if name[len(name)-len(CONFIG_SUFFIX):] != CONFIG_SUFFIX {
			continue
		}

		partialCfg := agent.PartialSettings{}
		partialPath := path.Join(settings.Main.Include, name)

		err := LoadTomlFile(partialPath, &partialCfg)
		if err != nil {
			return nil, err
		}

		//merge into settings
		for key, ext := range partialCfg.Extensions {
			_, m := settings.Extensions[key]
			_, p := partial.Extensions[key]
			if m || p {
				return nil, errors.New(fmt.Sprintf("Extension override in '%s' name '%s'", partialPath, key))
			}

			partial.Extensions[key] = ext
		}

		for key, startup := range partialCfg.Startup {
			_, m := settings.Startup[key]
			_, p := partial.Startup[key]
			if m || p {
				return nil, errors.New(fmt.Sprintf("Startup command override in '%s' name '%s'", partialPath, key))
			}

			partial.Startup[key] = startup
		}
	}

	return partial, nil
}

func GetSettings(filename string) *agent.Settings {
	settings := &agent.Settings{}

	//that's the main config file, panic if can't load
	if err := LoadTomlFile(filename, settings); err != nil {
		panic(err)
	}

	return settings
}
