package builtin

import (
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
}

type btrfsFS struct {
	Label        string
	UUID         string
	TotalDevices int
	Used         string
	Devices      []btrfsDevice
}

type btrfsDevice struct {
	DevID string
	Size  string
	Used  string
	Path  string
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
