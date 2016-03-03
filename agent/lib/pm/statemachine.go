package pm

import (
	"sync"
)

type StateMachine interface {
	Wait(key ...string) bool
	Release(key string, s bool)
}

type releaseReq struct {
	k string
	s bool
}

type waitReq struct {
	keys []string
	ch   chan bool
}

type stateMachineImpl struct {
	states map[string]bool

	waiting []waitReq
	rch     chan *releaseReq

	m sync.Mutex
}

func NewStateMachine() StateMachine {
	s := &stateMachineImpl{
		states:  make(map[string]bool),
		waiting: make([]waitReq, 0),
		rch:     make(chan *releaseReq),
	}

	go s.loop()

	return s
}

func (s *stateMachineImpl) loop() {
	for {
		r := <-s.rch
		s.states[r.k] = r.s

		//check waiting requests
		for i := len(s.waiting) - 1; i >= 0; i-- {
			wrq := s.waiting[i]
			state, ok := s.satisfied(wrq.keys)
			if !ok {
				//not all conditions has been released yet
				continue
			}
			//all the request key has been satisfied
			wrq.ch <- state

			s.m.Lock()
			s.waiting = append(s.waiting[:i], s.waiting[i+1:]...)
			s.m.Unlock()
		}
	}
}

func (s *stateMachineImpl) satisfied(keys []string) (state bool, ok bool) {
	state = true
	ok = true
	for _, k := range keys {
		var value bool
		if value, ok = s.states[k]; ok {
			state = state && value
		} else {
			ok = false
			state = false
			break
		}
	}

	return
}

func (s *stateMachineImpl) Wait(keys ...string) bool {
	if state, ok := s.satisfied(keys); ok {
		return state
	}
	wrq := waitReq{
		keys: keys,
		ch:   make(chan bool, 1),
	}

	defer func() {
		close(wrq.ch)
	}()

	s.m.Lock()
	s.waiting = append(s.waiting, wrq)
	s.m.Unlock()

	return <-wrq.ch
}

func (s *stateMachineImpl) Release(key string, state bool) {
	s.rch <- &releaseReq{
		k: key,
		s: state,
	}
}