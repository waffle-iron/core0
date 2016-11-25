package builtin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/g8os/core.base/pm"
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/process"
)

var (
	errBtrfsNoFS = errors.New("No btrfs FS found")
)

func init() {
	pm.CmdMap["btrfs.list"] = process.NewInternalProcessFactory(btrfsList)
	pm.CmdMap["btrfs.create"] = process.NewInternalProcessFactory(btrfsCreate)
}

type btrfsFS struct {
	Label        string        `json:"label"`
	UUID         string        `json:"uuid"`
	TotalDevices int           `json:"total_devices"`
	Used         string        `json:"used"`
	Devices      []btrfsDevice `json:"devices"`
}

type btrfsDevice struct {
	Missing bool   `json:"missing"`
	DevID   string `json:"dev_id"`
	Size    string `json:"size"`
	Used    string `json:"used"`
	Path    string `json:"path"`
}

var (
	// valid btrfs data & metadata profiles
	btrfsProfiles = map[string]struct{}{
		"raid0":  struct{}{},
		"raid1":  struct{}{},
		"raid5":  struct{}{},
		"raid6":  struct{}{},
		"raid10": struct{}{},
		"dup":    struct{}{},
		"single": struct{}{},
		"":       struct{}{},
	}
)

type btrfsCreateArgument struct {
	Label    string   `json:"label"`
	Metadata string   `json:"metadata"`
	Data     string   `json:"data"`
	Devices  []string `json:"devices"`
}

func (arg btrfsCreateArgument) Validate() error {
	if len(arg.Devices) == 0 {
		return fmt.Errorf("need to specify devices to create btrfs")
	}
	if v, ok := btrfsProfiles[arg.Metadata]; !ok {
		return fmt.Errorf("invalid metadata profile:%v", v)
	}
	if v, ok := btrfsProfiles[arg.Data]; !ok {
		return fmt.Errorf("invalid data profile:%v", v)
	}
	return nil
}

func btrfsCreate(cmd *core.Command) (interface{}, error) {
	var args btrfsCreateArgument
	var opts []string

	if err := json.Unmarshal(*cmd.Arguments, &args); err != nil {
		return nil, err
	}
	if err := args.Validate(); err != nil {
		return nil, err
	}

	if args.Label != "" {
		opts = append(opts, "-L", args.Label)
	}
	if args.Metadata != "" {
		opts = append(opts, "-m", args.Metadata)
	}
	if args.Data != "" {
		opts = append(opts, "-d", args.Data)
	}
	opts = append(opts, strings.Join(args.Devices, " "))

	out, err := exec.Command("mkfs.btrfs", opts...).Output()
	if err != nil {
		return string(out), err
	}
	return "OK", nil
}

// list btrfs FSs
func btrfsList(cmd *core.Command) (interface{}, error) {
	shellCmd := &core.Command{
		ID:      "12345",
		Command: process.CommandSystem,
		Arguments: core.MustArguments(
			process.SystemCommandArguments{
				Name: "btrfs",
				Args: []string{"filesystem", "show"},
			},
		),
	}

	runner, err := pm.GetManager().RunCmd(shellCmd)
	if err != nil {
		return "", err
	}

	result := runner.Wait()
	if result.State != core.StateSuccess || len(result.Streams) == 0 {
		return "", fmt.Errorf("error listing btrfs filesystem: %v", result.Streams)
	}

	fss, err := btrfsParseList(result.Streams[0])
	return fss, err
}

// parse `btrfs filesystem show` output
func btrfsParseList(output string) ([]btrfsFS, error) {
	var fss []btrfsFS

	all := strings.Split(output, "\n")
	if len(all) < 3 {
		return fss, errBtrfsNoFS
	}

	var fsLines []string
	for i, line := range all {
		line = strings.TrimSpace(line)

		// there are 3 markers of a filesystem
		// - empty line (original btrfs command)
		// - line started with `Label` and not first line (PM wrapped command)
		// - last line (original btrfs command & PM wrapped command)
		if (strings.HasPrefix(line, "Label") && i != 0) || line == "" || i == len(all)-1 {
			if !strings.HasPrefix(line, "Label") {
				fsLines = append(fsLines, line)
			}
			if len(fsLines) < 3 {
				continue
			}
			fs, err := btrfsParseFS(fsLines)
			if err != nil {
				return fss, err
			}
			fss = append(fss, fs)

			fsLines = []string{}
			if strings.HasPrefix(line, "Label") {
				fsLines = append(fsLines, line)
			}
		} else {
			fsLines = append(fsLines, line)
		}
	}
	return fss, nil
}

func btrfsParseFS(lines []string) (btrfsFS, error) {
	// first line should be label && uuid
	label, uuid := func(line string) (label string, uuid string) {
		parts := strings.Split(line, " ")
		if parts[0] == "Label:" {
			label = strings.TrimSpace(parts[1][1 : len(parts[1])-1])
		}
		if parts[3] == "uuid:" {
			uuid = strings.TrimSpace(parts[4])
		}
		return
	}(lines[0])

	// total device & byte used
	totDevice, used, err := func(line string) (totDevice int, used string, err error) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Total devices") {
			err = fmt.Errorf("Line 2 not prefixed with `Total Devices`")
			return
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "Total devices"))

		parts := strings.Split(line, "FS bytes used")
		if len(parts) != 2 {
			err = fmt.Errorf("invalid line :%v", line)
			return
		}
		totDevice, err = strconv.Atoi(strings.TrimSpace(parts[0]))
		used = strings.TrimSpace(parts[1])
		return
	}(lines[1])

	if err != nil {
		return btrfsFS{}, err
	}

	devs, err := btrfsParseDevices(lines[2:])
	if err != nil {
		return btrfsFS{}, err
	}
	return btrfsFS{
		Label:        label,
		UUID:         uuid,
		TotalDevices: totDevice,
		Used:         used,
		Devices:      devs,
	}, nil
}

func btrfsParseDevices(lines []string) ([]btrfsDevice, error) {
	var devs []btrfsDevice
	for _, line := range lines {
		if line == "" {
			continue
		}
		var dev btrfsDevice
		parts := strings.Fields(line)
		if len(parts) == 8 {
			dev = btrfsDevice{
				Missing: false,
				DevID:   parts[1],
				Size:    parts[3],
				Used:    parts[5],
				Path:    parts[7],
			}
		}
		devs = append(devs, dev)
	}
	return devs, nil
}
