package settings

import (
	"fmt"
	"sort"
)

type StartupTree interface {
	//Services a list of sorted startup services based on service weights.
	Services() []Startup

	//Slice gets a slice of the sorted start up processes that are lying between a
	//certain weight range. e can be set to -1 which means (end of list)
	//[s, e[
	Slice(s, e int64) []Startup
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

func (t *treeImpl) Slice(s, e int64) []Startup {
	slice := make([]Startup, 0)
	for i, p := range t.startups {
		w := t.weights[i]
		if w < s {
			continue
		}
		if w >= e && e != -1 {
			continue
		}

		slice = append(slice, p)
	}

	return slice
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
