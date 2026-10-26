// Package printer envía los bytes de una tirilla (ya generados por EOS) a una impresora de red o del sistema, y
// clasifica el resultado: Printed (aceptado), Rejected (no se aceptó nada: se puede reintentar sin duplicar) o
// Ambiguous (pudo imprimirse: EOS no lo reenvía solo).
package printer

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

type Result string

const (
	Printed   Result = "printed"
	Rejected  Result = "rejected"
	Ambiguous Result = "ambiguous"
)

// Target es una impresora tal como la describe EOS en el mensaje «hello».
type Target struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Connection string `json:"connection"` // network | system
	Host       string `json:"host"`
	Port       int    `json:"port"`
	SystemName string `json:"system_name"`
	Mode       string `json:"mode"`
	Cut        bool   `json:"cut"`
	Width      int    `json:"width"`
}

var (
	DialTimeout  = 5 * time.Second
	WriteTimeout = 10 * time.Second
	SpoolTimeout = 20 * time.Second
)

// Send imprime y devuelve el resultado con un mensaje legible para el personal.
func Send(t Target, data []byte) (Result, string) {
	switch t.Connection {
	case "network":
		return sendNetwork(t, data)
	case "system":
		if t.SystemName == "" {
			return Rejected, "La impresora no tiene una cola del sistema asignada."
		}
		return sendSystem(t.SystemName, data)
	default:
		return Rejected, fmt.Sprintf("Conexión desconocida: %q", t.Connection)
	}
}

func sendNetwork(t Target, data []byte) (Result, string) {
	port := t.Port
	if port == 0 {
		port = 9100
	}
	addr := net.JoinHostPort(t.Host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, DialTimeout)
	if err != nil {
		// Sin conexión no se envió ningún byte: es seguro reintentar.
		return Rejected, fmt.Sprintf("No responde en %s (%s).", addr, short(err))
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	written, err := conn.Write(data)
	if err != nil {
		if written == 0 && !isTimeout(err) {
			return Rejected, fmt.Sprintf("La impresora %s cerró la conexión sin recibir datos.", addr)
		}
		return Ambiguous, fmt.Sprintf("Se cortó el envío a %s tras %d de %d bytes: puede que se haya impreso.", addr, written, len(data))
	}
	return Printed, ""
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func short(err error) string {
	var op *net.OpError
	if errors.As(err, &op) && op.Err != nil {
		return op.Err.Error()
	}
	return err.Error()
}
