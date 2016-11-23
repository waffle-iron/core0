package pm

import (
	"github.com/g8os/core.base/pm/core"
	"github.com/g8os/core.base/pm/stream"
	"sync"
	"time"
)

type RunnerHook interface {
	Tick(delay time.Duration)
	Message(msg *stream.Message)
	Exit(state string)
	PID(pid int)
}

type NOOPHook struct {
}

func (h *NOOPHook) Tick(delay time.Duration)    {}
func (h *NOOPHook) Message(msg *stream.Message) {}
func (h *NOOPHook) Exit(state string)           {}
func (h *NOOPHook) PID(pid int)                 {}

type DelayHook struct {
	NOOPHook
	o sync.Once

	Delay  time.Duration
	Action func()
}

func (h *DelayHook) Tick(delay time.Duration) {
	if delay > h.Delay {
		h.o.Do(h.Action)
	}
}

type ExitHook struct {
	NOOPHook
	o sync.Once

	Action func(bool)
}

func (h *ExitHook) Exit(state string) {
	s := false
	if state == core.StateSuccess {
		s = true
	}

	h.o.Do(func() {
		h.Action(s)
	})
}

type PIDHook struct {
	NOOPHook
	o sync.Once

	Action func(pid int)
}

func (h *PIDHook) PID(pid int) {
	h.o.Do(func() {
		h.Action(pid)
	})
}

type MatchHook struct {
	NOOPHook
	Match string
	o     sync.Once

	Action func(msg *stream.Message)
}

func (h *MatchHook) Message(msg *stream.Message) {
	if msg.Message == h.Match {
		h.o.Do(func() {
			h.Action(msg)
		})
	}
}
