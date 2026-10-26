package config

import (
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// DeviceName es el nombre del equipo que se muestra en EOS: el que ve el usuario en su sistema («MacBook Air de Ana»,
// «CAJA-01»), no el nombre de red, que en muchas redes es solo la IP («192.168.1.12»).
func DeviceName() string {
	switch runtime.GOOS {
	case "darwin":
		if out, err := exec.Command("scutil", "--get", "ComputerName").Output(); err == nil {
			if name := strings.TrimSpace(string(out)); name != "" {
				return name
			}
		}
	case "windows":
		if name := strings.TrimSpace(os.Getenv("COMPUTERNAME")); name != "" {
			return name
		}
	}
	host, _ := os.Hostname()
	host = strings.TrimSuffix(strings.TrimSuffix(host, ".local"), ".lan")
	if host == "" || net.ParseIP(host) != nil {
		return "Equipo de impresión"
	}
	return host
}
