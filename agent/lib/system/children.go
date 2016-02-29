package system

import (
	"os"
	"os/signal"
	"syscall"
)

func defunct() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGCHLD)
	for _ = range ch {
		var status syscall.WaitStatus
		var rusage syscall.Rusage
		syscall.Wait4(-1, &status, syscall.WNOHANG, &rusage)
	}
}

func CollectDefunct() {
	go defunct()
}
