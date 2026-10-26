// Package service instala el agente como servicio del sistema y gestiona su archivo de log.
package service

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"eos-print-agent/internal/config"
)

const maxLogSize = 5 << 20 // 5 MB; se conserva un archivo anterior (agent.log.1)

func LogPath() string { return filepath.Join(config.Dir(), "agent.log") }

type rotating struct {
	mu   sync.Mutex
	file *os.File
	size int64
}

func (r *rotating) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil || r.size+int64(len(p)) > maxLogSize {
		if r.file != nil {
			r.file.Close()
			_ = os.Rename(LogPath(), LogPath()+".1")
		}
		if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
			return 0, err
		}
		f, err := os.OpenFile(LogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return 0, err
		}
		info, _ := f.Stat()
		r.file, r.size = f, info.Size()
	}
	n, err := r.file.Write(p)
	r.size += int64(n)
	return n, err
}

// Logger escribe en el archivo de log y, si se pide, también en la consola.
func Logger(console bool) *log.Logger {
	var out io.Writer = &rotating{}
	if console {
		out = io.MultiWriter(out, os.Stderr)
	}
	return log.New(out, "", log.LstdFlags)
}
