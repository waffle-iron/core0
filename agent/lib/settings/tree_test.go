package settings

import (
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"sort"
	"testing"
)

var (
	settings IncludedSettings = IncludedSettings{
		Startups: map[string]Startup{
			"ovc": Startup{
				key:   "ovc",
				After: []After{"mongo", "influx"},
			},
			"fstab": Startup{
				key:   "fstab",
				After: []After{AfterInit, "udev"},
			},
			"mongo": Startup{
				key:   "mongo",
				After: []After{AfterBoot},
			},
			"sshd": Startup{
				key:   "sshd",
				After: []After{AfterNet},
			},
			"influx": Startup{
				key:   "influx",
				After: []After{AfterBoot},
			},
			"udev": Startup{
				key:   "udev",
				After: []After{AfterInit},
			},
		},
	}
)

func TestMain(m *testing.M) {

	os.Exit(m.Run())
}

func TestGetTree(t *testing.T) {
	tree, errors := settings.GetStartupTree()
	if ok := assert.Empty(t, errors); !ok {
		t.Fail()
	}

	if ok := assert.NotNil(t, tree); !ok {
		t.Fatal()
	}
}

func AssertTree(t *testing.T, expected [][]string, tree StartupTree) bool {
	services := tree.Services()
	offset := 0
	ok := true
	for i, e := range expected {
		slice := services[offset : offset+len(e)]
		actual := make([]string, 0)
		for _, s := range slice {
			actual = append(actual, s.Key())
		}

		sort.Strings(e)
		sort.Strings(actual)

		if !assert.Equal(t, e, actual) {
			t.Fail()
			ok = false
		}
		offset = i + len(e)
	}

	return ok
}

func TestGetTreeSorted(t *testing.T) {
	tree, errors := settings.GetStartupTree()
	if ok := assert.Empty(t, errors); !ok {
		t.Fail()
	}

	if ok := assert.NotNil(t, tree); !ok {
		t.Fatal()
	}

	//expected order
	expected := [][]string{
		[]string{"udev"},
		[]string{"fstab"},
		[]string{"sshd"},
		[]string{"mongo", "influx"},
		[]string{"ovc"}}

	if !AssertTree(t, expected, tree) {
		t.Fatal()
	}
}

func TestGetTreeDefaultWeight(t *testing.T) {
	settings := IncludedSettings{
		Startups: map[string]Startup{
			"mount": Startup{
				key:   "mount",
				After: []After{AfterInit},
			},
			"mongo": Startup{
				key:   "mongo",
				After: []After{},
			},
			"influx": Startup{
				key:   "influx",
				After: []After{},
			},
		},
	}

	tree, errors := settings.GetStartupTree()
	if ok := assert.Empty(t, errors); !ok {
		t.Fail()
	}

	if ok := assert.NotNil(t, tree); !ok {
		t.Fatal()
	}

	//expected order
	expected := [][]string{
		[]string{"mount"},
		[]string{"mongo", "influx"},
	}

	if !AssertTree(t, expected, tree) {
		t.Fatal()
	}
}

func TestGetTreeMissingDependency(t *testing.T) {
	settings := IncludedSettings{
		Startups: map[string]Startup{
			"mongo": Startup{
				key:   "mongo",
				After: []After{AfterBoot},
			},
			"ovc": Startup{
				key:   "ovc",
				After: []After{"mongo", "influx"},
			},
		},
	}

	tree, errors := settings.GetStartupTree()
	if ok := assert.NotEmpty(t, errors); !ok {
		t.Fail()
	}

	log.Println(errors)

	if ok := assert.Len(t, tree.Services(), 1); !ok {
		t.Fatal()
	}
}

func TestGetTreeCyclicDependency(t *testing.T) {
	settings := IncludedSettings{
		Startups: map[string]Startup{
			"independent": Startup{
				key:   "independent",
				After: []After{},
			},
			"influx": Startup{
				key:   "influx",
				After: []After{},
			},
			"mongo": Startup{
				key:   "mongo",
				After: []After{"ovc"},
			},
			"ovc": Startup{
				key:   "ovc",
				After: []After{"mongo", "influx"},
			},
		},
	}

	tree, errors := settings.GetStartupTree()
	if ok := assert.NotEmpty(t, errors); !ok {
		t.Fail()
	}

	log.Println(errors)

	if ok := assert.Len(t, tree.Services(), 2); !ok {
		t.Fatal()
	}
}
