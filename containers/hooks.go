package containers

type hooks struct {
	mgr  *containerManager
	root string

	coreID string
	pid    int
}

func (h *hooks) onPID(pid int) {
	h.pid = pid
	h.mgr.postStart(h.coreID, pid)
}

func (h *hooks) onExit(state bool) {
	log.Debugf("Container %s exited with state %v", h.coreID, state)
	h.mgr.cleanup(h.coreID, h.pid, h.root)
}
