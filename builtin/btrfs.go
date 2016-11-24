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
	DevID string `json:"dev_id"`
	Size  string `json:"size"`
	Used  string `json:"used"`
	Path  string `json:"path"`
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
	out, err := exec.Command("btrfs", "filesystem", "show").Output()
	if err != nil {
		return "", err
	}
	fss, err := btrfsParseList(out)
	if err == errBtrfsNoFS {
		err = nil
	}
	return fss, err
}

// parse `btrfs filesystem show` output
func btrfsParseList(output []byte) ([]btrfsFS, error) {
	var fss []btrfsFS

	all := strings.Split(string(output), "\n")

	if len(all) < 3 {
		return fss, errBtrfsNoFS
	}

	var fsLines []string
	for i, line := range all {
		line = strings.TrimSpace(line)
		if line == "" || i == len(all)-1 {
			if len(fsLines) < 3 { // empty line, ignore it
				break
			}
			fs, err := btrfsParseFS(fsLines)
			if err != nil {
				return fss, err
			}
			fss = append(fss, fs)
			fsLines = []string{}
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
		parts := strings.Fields(line)
		if len(parts) != 8 {
			return devs, fmt.Errorf("invalid device line:%v", line)
		}
		dev := btrfsDevice{
			DevID: parts[1],
			Size:  parts[3],
			Used:  parts[5],
			Path:  parts[7],
		}
		devs = append(devs, dev)
	}
	return devs, nil
}
