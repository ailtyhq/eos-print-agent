//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
)

const (
	unitPath    = "/etc/systemd/system/eos-print-agent.service"
	serviceUser = "eos-print"
	dataDir     = "/var/lib/eos-print-agent"
)

// En Linux es un servicio de systemd con un usuario propio sin login, en el grupo `lp` (permiso mínimo para imprimir).
// La configuración vive en /var/lib/eos-print-agent, legible solo por ese usuario.
func Install(exe string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("ejecuta la instalación con sudo")
	}
	if _, err := user.Lookup(serviceUser); err != nil {
		if out, err := exec.Command("useradd", "--system", "--no-create-home", "--shell", "/usr/sbin/nologin",
			"--groups", "lp", serviceUser).CombinedOutput(); err != nil {
			return fmt.Errorf("useradd: %s", out)
		}
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	if out, err := exec.Command("chown", "-R", serviceUser+":"+serviceUser, dataDir).CombinedOutput(); err != nil {
		return fmt.Errorf("chown: %s", out)
	}
	unit := fmt.Sprintf(`[Unit]
Description=EOS Print Agent
After=network-online.target cups.service
Wants=network-online.target

[Service]
User=%s
Environment=EOS_PRINT_AGENT_HOME=%s
ExecStart=%s run
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=%s

[Install]
WantedBy=multi-user.target
`, serviceUser, dataDir, exe, dataDir)
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{{"daemon-reload"}, {"enable", "--now", "eos-print-agent"}} {
		if out, err := exec.Command("systemctl", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl %v: %s", args, out)
		}
	}
	return nil
}

func Uninstall() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("ejecuta la desinstalación con sudo")
	}
	_ = exec.Command("systemctl", "disable", "--now", "eos-print-agent").Run()
	_ = os.Remove(unitPath)
	return exec.Command("systemctl", "daemon-reload").Run()
}

func Status() string {
	out, _ := exec.Command("systemctl", "is-active", "eos-print-agent").Output()
	return string(out)
}

func Stop() error  { return exec.Command("systemctl", "stop", "eos-print-agent").Run() }
func Start() error { return exec.Command("systemctl", "start", "eos-print-agent").Run() }

// DataDir es donde «pair» debe guardar la configuración del servicio cuando se ejecuta con sudo.
func DataDir() string { return dataDir }

func RunService(run func(stop <-chan struct{}) error) error { return run(signals()) }
