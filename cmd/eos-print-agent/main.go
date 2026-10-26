// eos-print-agent: agente de impresión de comandas de EOS (HU 20.8d).
//
//	eos-print-agent pair --url https://<tu-negocio>.eos... --code ABCD-1234 [--name "Mac de cocina"]
//	eos-print-agent install | uninstall | start | stop | status | logs | run | unpair | version
//	eos-print-agent test --host 192.168.1.50 [--port 9100] | --queue NOMBRE [--width 58]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"eos-print-agent/internal/agent"
	"eos-print-agent/internal/config"
	"eos-print-agent/internal/printer"
	"eos-print-agent/internal/service"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "pair":
		err = pair(args)
	case "run":
		err = run()
	case "install":
		err = install()
	case "uninstall":
		err = service.Uninstall()
	case "start":
		err = service.Start()
	case "stop":
		err = service.Stop()
	case "status":
		status()
	case "logs":
		err = logs()
	case "test":
		err = test(args)
	case "unpair":
		err = config.Remove()
	case "version", "--version":
		fmt.Printf("eos-print-agent %s (protocolo %d)\n", agent.Version, agent.Protocol)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Agente de impresión de EOS

  pair --url <dirección de EOS> --code <código>   Vincula este equipo (el código se genera en EOS → Impresoras)
  install                                          Lo instala como servicio y lo arranca con el equipo
  status | logs | start | stop | uninstall         Estado, últimas líneas del log, controlar o quitar el servicio
  test --host <IP> [--port 9100] | --queue <cola>  Imprime una tirilla de prueba sin pasar por EOS
  unpair | version
`)
}

func pair(args []string) error {
	fs := flag.NewFlagSet("pair", flag.ExitOnError)
	rawURL := fs.String("url", "", "dirección de EOS (https://tu-negocio...)")
	code := fs.String("code", "", "código de vinculación")
	host, _ := os.Hostname()
	name := fs.String("name", host, "nombre del equipo en EOS")
	_ = fs.Parse(args)
	if *rawURL == "" || *code == "" {
		return errors.New("faltan --url y --code")
	}
	base, err := config.NormalizeURL(*rawURL)
	if err != nil {
		return err
	}
	cfg, err := agent.Pair(base, *code, *name)
	if err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("Equipo vinculado a la sucursal %s (token %s). Siguiente paso: eos-print-agent install\n", cfg.BranchName, config.Hint(cfg.Token))
	return nil
}

func run() error {
	return service.RunService(func(stop <-chan struct{}) error {
		logger := service.Logger(isTerminal())
		cfg, err := config.Load()
		if err != nil {
			logger.Print(err)
			return err
		}
		logger.Printf("eos-print-agent %s iniciando; EOS %s (token %s)", agent.Version, cfg.URL, config.Hint(cfg.Token))
		ctx, cancel := context.WithCancel(context.Background())
		go func() { <-stop; cancel() }()
		err = agent.New(cfg, logger).Run(ctx)
		logger.Printf("agente detenido: %v", err)
		if errors.Is(err, agent.ErrStop) {
			return nil // salida «correcta»: launchd/systemd no lo reinician en bucle con un token revocado
		}
		return err
	})
}

func install() error {
	if _, err := config.Load(); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	if err := service.Install(exe); err != nil {
		return err
	}
	fmt.Println("Servicio instalado y en ejecución. Revisa el estado en EOS → Impresoras o con: eos-print-agent status")
	return nil
}

func status() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("Vinculación:", err)
	} else {
		fmt.Printf("Vinculado a %s, sucursal %s (token %s)\n", cfg.URL, cfg.BranchName, config.Hint(cfg.Token))
	}
	fmt.Println("Servicio:", strings.TrimSpace(service.Status()))
	fmt.Println("Log:", service.LogPath())
	if names := printer.SystemPrinters(); len(names) > 0 {
		fmt.Println("Impresoras del sistema:", strings.Join(names, ", "))
	}
}

func logs() error {
	data, err := os.ReadFile(service.LogPath())
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > 50 {
		lines = lines[len(lines)-50:]
	}
	fmt.Println(strings.Join(lines, "\n"))
	return nil
}

func test(args []string) error {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	host := fs.String("host", "", "IP de la impresora de red")
	port := fs.Int("port", 9100, "puerto")
	queue := fs.String("queue", "", "cola de impresión del sistema (USB)")
	width := fs.Int("width", 80, "ancho del papel (80 o 58)")
	text := fs.Bool("text", false, "sin comandos ESC/POS (impresoras de texto plano)")
	_ = fs.Parse(args)
	target := printer.Target{Name: "prueba", Port: *port, Width: *width, Cut: !*text}
	switch {
	case *host != "":
		target.Connection, target.Host = "network", *host
	case *queue != "":
		target.Connection, target.SystemName = "system", *queue
	default:
		return errors.New("indica --host o --queue")
	}
	result, detail := printer.Send(target, printer.TestTicket(*width, !*text))
	fmt.Println(result, detail)
	if result != printer.Printed {
		return errors.New("la prueba no se imprimió")
	}
	return nil
}

// isTerminal: en una consola el log también se muestra en pantalla; como servicio, solo en el archivo.
func isTerminal() bool {
	info, err := os.Stderr.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
