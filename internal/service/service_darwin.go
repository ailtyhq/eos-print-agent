//go:build darwin

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

const label = "com.eos.printagent"

// En macOS es un LaunchAgent del usuario (no un daemon de root): las colas de CUPS y la impresora USB son de su sesión.
func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

func domain() string { return "gui/" + strconv.Itoa(os.Getuid()) }

func Install(exe string) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("en macOS instálalo sin sudo, con el usuario que usa la impresora")
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key><array><string>%s</string><string>run</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
  <key>ProcessType</key><string>Background</string>
</dict>
</plist>
`, label, exe)
	if err := os.MkdirAll(filepath.Dir(plistPath()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(plistPath(), []byte(plist), 0o644); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "bootout", domain()+"/"+label).Run()
	if out, err := exec.Command("launchctl", "bootstrap", domain(), plistPath()).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %s", out)
	}
	return nil
}

func Uninstall() error {
	_ = exec.Command("launchctl", "bootout", domain()+"/"+label).Run()
	err := os.Remove(plistPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func Status() string {
	if err := exec.Command("launchctl", "print", domain()+"/"+label).Run(); err != nil {
		return "no instalado o detenido"
	}
	return "en ejecución (launchctl " + domain() + "/" + label + ")"
}

func Stop() error  { return exec.Command("launchctl", "bootout", domain()+"/"+label).Run() }
func Start() error { return exec.Command("launchctl", "bootstrap", domain(), plistPath()).Run() }

// RunService ejecuta el agente en primer plano (launchd lo vigila).
func RunService(run func(stop <-chan struct{}) error) error { return run(signals()) }
