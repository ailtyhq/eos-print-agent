//go:build darwin || linux

package printer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// sendSystem envía a una cola de CUPS en crudo (`lp -o raw`): la cola debe aceptar datos raw (ESC/POS o texto).
// `lp` termina con error antes de encolar si la cola no existe o no acepta trabajos (Rejected); si se vence el tiempo
// no se sabe si quedó encolado (Ambiguous).
func sendSystem(queue string, data []byte) (Result, string) {
	if paused, detail := queuePaused(queue); paused {
		// Encolar en una cola en pausa la «aceptaría» sin imprimir: se rechaza antes de enviar.
		return Rejected, detail
	}
	ctx, cancel := context.WithTimeout(context.Background(), SpoolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lp", "-d", queue, "-o", "raw")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C") // mensajes de lp en inglés, sin importar el idioma del equipo
	cmd.Stdin = bytes.NewReader(data)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return Ambiguous, fmt.Sprintf("La cola %q no respondió a tiempo: puede que se haya impreso.", queue)
	}
	if err != nil {
		return Rejected, fmt.Sprintf("La cola %q rechazó el trabajo: %s", queue, strings.TrimSpace(errOut.String()+" "+err.Error()))
	}
	return classifyLp(out.String(), queue)
}

// classifyLp: `lp` terminó con código 0, así que CUPS encoló el trabajo (con LC_ALL=C lo confirma con «request id
// is <cola>-<n>»). El código de salida es la señal: el texto solo se usa para el log.
func classifyLp(stdout, _ string) (Result, string) {
	return Printed, strings.TrimSpace(stdout)
}

// queuePaused: `lpstat -p` dice «disabled» si la cola está en pausa. macOS responde en el idioma del sistema aunque
// se pida inglés, así que también se reconoce el español («desactivada»).
func queuePaused(queue string) (bool, string) {
	cmd := exec.Command("lpstat", "-p", queue)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	out, err := cmd.Output()
	if err != nil {
		return false, "" // sin información: que decida `lp`
	}
	return isDisabled(string(out)), fmt.Sprintf("La cola %q está en pausa: reactívala en Ajustes → Impresoras (o con cupsenable).", queue)
}

func isDisabled(lpstat string) bool {
	text := strings.ToLower(lpstat)
	return strings.Contains(text, "disabled") || strings.Contains(text, "desactivad")
}

// SystemPrinters lista las colas del sistema (`lpstat -e`), para elegirlas en EOS.
func SystemPrinters() []string {
	out, err := exec.Command("lpstat", "-e").Output()
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			names = append(names, name)
		}
	}
	return names
}
