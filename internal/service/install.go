package service

import (
	"io"
	"os"
	"path/filepath"
)

// CopySelf deja una copia del ejecutable en BinPath (si no se está ejecutando ya desde ahí) y devuelve esa ruta.
func CopySelf(exe string) (string, error) {
	target := BinPath()
	if same, _ := filepath.Abs(exe); same == target {
		return target, nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	src, err := os.Open(exe)
	if err != nil {
		return "", err
	}
	defer src.Close()
	tmp := target + ".new"
	dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return "", err
	}
	if err := dst.Close(); err != nil {
		return "", err
	}
	_ = Uninstall() // el servicio anterior suelta el archivo antes de reemplazarlo (Windows lo bloquea en uso)
	return target, os.Rename(tmp, target)
}
