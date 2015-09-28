package utils

import (
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

func GetSettings(filename string) agent.Settings {
	settings := agent.Settings{}

	LoadTomlFile(filename, &settings)
	if settings.Main.Include == "" {
		return settings
	}

	infos, err := ioutil.ReadDir(settings.Main.Include)
	if err != nil {
		panic(err)
	}

	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		name := info.Name()

		if name[len(name)-len(CONFIG_SUFFIX):] != CONFIG_SUFFIX {
			continue
		}

		partial := agent.PartialSettings{}
		partialPath := path.Join(settings.Main.Include, name)

		LoadTomlFile(partialPath, &partial)

		//merge into settings
		for key, ext := range partial.Extensions {
			_, ok := settings.Extensions[key]
			if ok {
				panic(fmt.Sprintf("Extension override in '%s' name '%s'", partialPath, key))
			}

			settings.Extensions[key] = ext
		}

		for key, startup := range partial.Startup {
			_, ok := settings.Startup[key]
			if ok {
				panic(fmt.Sprintf("Startup command override in '%s' name '%s'", partialPath, key))
			}

			settings.Startup[key] = startup
		}
	}

	return settings
}
