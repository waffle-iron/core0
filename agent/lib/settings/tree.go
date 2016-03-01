package settings

import (
	"fmt"
	"sort"
)

type StartupTree interface {
	Services() []Startup
}

type treeImpl struct {
	startups []Startup
	weights  map[int]int64
}

func (t *treeImpl) Services() []Startup {
	return t.startups
}

func (t *treeImpl) Len() int {
	return len(t.startups)
}

func (t *treeImpl) Less(i, j int) bool {
	return t.weights[i] < t.weights[j]
}

func (t *treeImpl) Swap(i, j int) {
	t.startups[i], t.startups[j] = t.startups[j], t.startups[i]
	t.weights[i], t.weights[j] = t.weights[j], t.weights[i]
}

func (i *IncludedSettings) GetStartupTree() (StartupTree, []error) {
	var errors []error
	tree := &treeImpl{
		startups: make([]Startup, 0, len(i.Startups)),
		weights:  make(map[int]int64),
	}

	for _, s := range i.Startups {
		w, err := s.Weight(i)
		if err != nil {
			//we don't return on error because a broken service MUST NOT bring the whole system
			//down.
			errors = append(errors, fmt.Errorf("Can't calculate service '%s' weights because of %s", s, err))
			continue
		}

		tree.startups = append(tree.startups, s)
		tree.weights[len(tree.startups)-1] = w
	}

	sort.Sort(tree)

	return tree, errors
}
