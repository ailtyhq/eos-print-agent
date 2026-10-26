package service

import (
	"os"
	"os/signal"
	"syscall"
)

// signals cierra el canal al recibir Ctrl+C o SIGTERM (systemd/launchd al detener el servicio).
func signals() <-chan struct{} {
	stop := make(chan struct{})
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() { <-ch; close(stop) }()
	return stop
}
