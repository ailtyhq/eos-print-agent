//go:build windows

package service

import (
	"fmt"
	"os/exec"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"eos-print-agent/internal/config"
)

const name = "EOSPrintAgent"

// En Windows es un servicio con la cuenta LocalService (permisos mínimos) y arranque automático. La carpeta de
// configuración (ProgramData\EOS\PrintAgent) queda solo para SYSTEM, Administradores y LocalService.
func Install(exe string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("abre la consola como administrador: %w", err)
	}
	defer m.Disconnect()
	if s, err := m.OpenService(name); err == nil {
		s.Close()
		_ = Uninstall()
		time.Sleep(2 * time.Second)
	}
	s, err := m.CreateService(name, exe, mgr.Config{
		DisplayName:      "EOS Print Agent",
		Description:      "Imprime las comandas de EOS en las impresoras del local.",
		StartType:        mgr.StartAutomatic,
		ServiceStartName: `NT AUTHORITY\LocalService`,
	}, "run")
	if err != nil {
		return err
	}
	defer s.Close()
	_ = s.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 5 * time.Second}}, 86400)
	if out, err := exec.Command("icacls", config.Dir(), "/inheritance:r", "/grant:r",
		"*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F", "*S-1-5-19:(OI)(CI)M").CombinedOutput(); err != nil {
		return fmt.Errorf("icacls: %s", out)
	}
	return s.Start()
}

func Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return nil
	}
	defer s.Close()
	_, _ = s.Control(svc.Stop)
	return s.Delete()
}

func Status() string {
	m, err := mgr.Connect()
	if err != nil {
		return "desconocido (sin permisos)"
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return "no instalado"
	}
	defer s.Close()
	st, err := s.Query()
	if err != nil {
		return "desconocido"
	}
	return map[svc.State]string{svc.Running: "en ejecución", svc.Stopped: "detenido"}[st.State]
}

func Stop() error  { return exec.Command("sc.exe", "stop", name).Run() }
func Start() error { return exec.Command("sc.exe", "start", name).Run() }

type handler struct {
	run func(stop <-chan struct{}) error
}

func (h handler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- h.run(stop) }()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case req := <-requests:
			if req.Cmd == svc.Stop || req.Cmd == svc.Shutdown {
				status <- svc.Status{State: svc.StopPending}
				close(stop)
				<-done
				return false, 0
			}
			status <- req.CurrentStatus
		case <-done:
			return false, 1
		}
	}
}

// RunService usa el gestor de servicios de Windows si corre como servicio; si no, en primer plano.
func RunService(run func(stop <-chan struct{}) error) error {
	if is, _ := svc.IsWindowsService(); is {
		return svc.Run(name, handler{run})
	}
	return run(signals())
}
