// Package agent mantiene la conexión con EOS (`ws/print-agent/`), imprime los trabajos y confirma cada resultado.
package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"eos-print-agent/internal/config"
	"eos-print-agent/internal/printer"
)

const Protocol = 1

var Version = "dev" // se fija al compilar (-ldflags "-X eos-print-agent/internal/agent.Version=1.0.0")

// Códigos de cierre del servidor que no se arreglan reconectando.
const (
	closeUnauthorized = 4401
	closeForbidden    = 4403
	closeProtocol     = 4426
)

// ErrStop indica que reconectar no sirve (token revocado, plan sin restaurante o protocolo viejo).
var ErrStop = errors.New("detenido")

type message struct {
	Event    string           `json:"event"`
	Agent    *agentInfo       `json:"agent,omitempty"`
	Printers []printer.Target `json:"printers,omitempty"`
	Job      *job             `json:"job,omitempty"`
}

type agentInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	BranchName string `json:"branch_name"`
}

type job struct {
	ID      string `json:"id"`
	Printer string `json:"printer"`
	Kind    string `json:"kind"`
	Data    string `json:"data"`
}

// Sender permite sustituir la impresión en las pruebas.
type Sender func(printer.Target, []byte) (printer.Result, string)

type Agent struct {
	Config *config.Config
	Logger *log.Logger
	Send   Sender

	mu       sync.Mutex
	printers map[string]printer.Target
}

func New(cfg *config.Config, logger *log.Logger) *Agent {
	return &Agent{Config: cfg, Logger: logger, Send: printer.Send, printers: map[string]printer.Target{}}
}

// Run conecta y reconecta (con espera creciente, hasta 60 s) hasta que el contexto termina o el servidor lo detiene.
func (a *Agent) Run(ctx context.Context) error {
	wait := time.Second
	for {
		started := time.Now()
		err := a.session(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, ErrStop) {
			return err
		}
		if time.Since(started) > time.Minute {
			wait = time.Second
		}
		a.Logger.Printf("conexión perdida (%v); reintento en %s", err, wait)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
		if wait *= 2; wait > time.Minute {
			wait = time.Minute
		}
	}
}

func wsURL(base string) (string, string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", "", err
	}
	origin := u.Scheme + "://" + u.Host
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = "/ws/print-agent/"
	return u.String(), origin, nil
}

func (a *Agent) session(ctx context.Context) error {
	target, origin, err := wsURL(a.Config.URL)
	if err != nil {
		return err
	}
	header := http.Header{}
	header.Set("Authorization", "Bearer "+a.Config.Token)
	header.Set("Origin", origin) // lo exige el validador de orígenes del servidor
	header.Set("X-EOS-Agent", fmt.Sprintf("%s; protocol=%d; os=%s", Version, Protocol, runtime.GOOS))
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	conn, _, err := websocket.Dial(dialCtx, target, &websocket.DialOptions{HTTPHeader: header})
	cancel()
	if err != nil {
		return fmt.Errorf("no se pudo conectar a %s: %w", origin, err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(4 << 20)

	sessionCtx, stop := context.WithCancel(ctx)
	defer stop()
	go a.keepAlive(sessionCtx, conn)

	for {
		var msg message
		if err := wsjson.Read(sessionCtx, conn, &msg); err != nil {
			switch websocket.CloseStatus(err) {
			case closeUnauthorized:
				a.Logger.Printf("EOS rechazó el token (revocado o inválido): vuelve a vincular el equipo")
				return ErrStop
			case closeForbidden:
				a.Logger.Printf("el plan del comercio no incluye el módulo de restaurante")
				return ErrStop
			case closeProtocol:
				a.Logger.Printf("esta versión del agente (%s) ya no es compatible: instala la última", Version)
				return ErrStop
			}
			return err
		}
		if err := a.handle(sessionCtx, conn, msg); err != nil {
			return err
		}
	}
}

func (a *Agent) keepAlive(ctx context.Context, conn *websocket.Conn) {
	ping := time.NewTicker(30 * time.Second)
	report := time.NewTicker(5 * time.Minute)
	defer ping.Stop()
	defer report.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			_ = a.write(ctx, conn, map[string]any{"type": "ping"})
		case <-report.C:
			a.reportPrinters(ctx, conn)
		}
	}
}

var writeMu sync.Mutex

func (a *Agent) write(ctx context.Context, conn *websocket.Conn, v any) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return wsjson.Write(wctx, conn, v)
}

func (a *Agent) reportPrinters(ctx context.Context, conn *websocket.Conn) {
	_ = a.write(ctx, conn, map[string]any{"type": "system_printers", "printers": printer.SystemPrinters()})
}

func (a *Agent) handle(ctx context.Context, conn *websocket.Conn, msg message) error {
	switch msg.Event {
	case "hello", "printers_changed":
		a.setPrinters(msg.Printers)
		if msg.Agent != nil {
			a.Logger.Printf("conectado como %q (sucursal %s) con %d impresora(s)", msg.Agent.Name, msg.Agent.BranchName, len(msg.Printers))
		}
		a.reportPrinters(ctx, conn)
	case "job":
		if msg.Job != nil {
			return a.write(ctx, conn, a.print(*msg.Job))
		}
	case "revoked":
		a.Logger.Printf("EOS revocó este equipo")
		return ErrStop
	}
	return nil
}

func (a *Agent) setPrinters(list []printer.Target) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.printers = map[string]printer.Target{}
	for _, p := range list {
		a.printers[p.ID] = p
	}
}

// print imprime un trabajo y arma su confirmación. Nunca registra los datos de la tirilla.
func (a *Agent) print(j job) map[string]any {
	ack := map[string]any{"type": "ack", "job": j.ID}
	a.mu.Lock()
	target, ok := a.printers[j.Printer]
	a.mu.Unlock()
	data, err := base64.StdEncoding.DecodeString(j.Data)
	var result printer.Result
	var detail string
	switch {
	case !ok:
		result, detail = printer.Rejected, "Este equipo no tiene esa impresora."
	case err != nil || len(bytes.TrimSpace(data)) == 0:
		result, detail = printer.Rejected, "La tirilla llegó vacía o dañada."
	default:
		result, detail = a.Send(target, data)
	}
	ack["status"], ack["error"] = string(result), detail
	a.Logger.Printf("trabajo %s (%s) en %q: %s %s", shortID(j.ID), j.Kind, target.Name, result, detail)
	return ack
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// Pair canjea el código de vinculación por el token del equipo.
func Pair(baseURL, code, name string) (*config.Config, error) {
	body, _ := json.Marshal(map[string]string{"code": code, "name": name, "platform": runtime.GOOS, "version": Version})
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Post(strings.TrimRight(baseURL, "/")+"/api/print-agents/pair/", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("no se pudo contactar a EOS: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		AgentID    string `json:"agent_id"`
		Token      string `json:"token"`
		Branch     string `json:"branch"`
		BranchName string `json:"branch_name"`
		Protocol   int    `json:"protocol"`
		Error      string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != http.StatusCreated {
		if out.Error == "" {
			out.Error = resp.Status
		}
		return nil, errors.New(out.Error)
	}
	if out.Protocol != Protocol {
		return nil, fmt.Errorf("EOS usa el protocolo %d y este agente el %d: instala la versión correspondiente", out.Protocol, Protocol)
	}
	return &config.Config{URL: baseURL, Token: out.Token, AgentID: out.AgentID, Branch: out.Branch, BranchName: out.BranchName}, nil
}
