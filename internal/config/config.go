// Package config guarda la vinculación del agente (URL del tenant y token) en un archivo legible solo por su dueño.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Config es la vinculación del equipo con una sucursal de un tenant de EOS.
type Config struct {
	URL        string `json:"url"`
	Token      string `json:"token"`
	AgentID    string `json:"agent_id"`
	Branch     string `json:"branch"`
	BranchName string `json:"branch_name"`
}

// Dir es la carpeta de configuración y logs. EOS_PRINT_AGENT_HOME la reemplaza (pruebas o instalaciones especiales).
func Dir() string {
	if dir := os.Getenv("EOS_PRINT_AGENT_HOME"); dir != "" {
		return dir
	}
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("ProgramData"), "EOS", "PrintAgent")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "EOS Print Agent")
	default:
		if os.Geteuid() == 0 {
			return "/var/lib/eos-print-agent"
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "eos-print-agent")
	}
}

func path() string { return filepath.Join(Dir(), "config.json") }

// Load lee la vinculación; ErrNotPaired si el equipo no se ha vinculado.
func Load() (*Config, error) {
	data, err := os.ReadFile(path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotPaired
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("configuración dañada: %w", err)
	}
	if c.URL == "" || c.Token == "" {
		return nil, ErrNotPaired
	}
	return &c, nil
}

// Save escribe la vinculación con permisos 0600 (el token no debe leerlo otro usuario del equipo).
func Save(c *Config) error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path())
}

// Remove borra la vinculación (al desvincular el equipo).
func Remove() error {
	err := os.Remove(path())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

var ErrNotPaired = errors.New("este equipo no está vinculado: ejecuta «eos-print-agent pair»")

// NormalizeURL deja solo esquema y host. Exige https salvo en localhost (desarrollo).
func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("dirección inválida: %q", raw)
	}
	host := u.Hostname()
	local := host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "127.0.0.1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return "", errors.New("la dirección debe empezar por https://")
	}
	return u.Scheme + "://" + u.Host, nil
}

// Hint muestra solo el final del token (nunca se escribe completo en pantalla ni en logs).
func Hint(token string) string {
	if len(token) <= 4 {
		return "…"
	}
	return "…" + token[len(token)-4:]
}
