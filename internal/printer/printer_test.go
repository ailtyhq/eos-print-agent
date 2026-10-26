package printer

import (
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

func listen(t *testing.T, handle func(net.Conn)) (string, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handle(conn)
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

func TestNetworkPrinted(t *testing.T) {
	got := make(chan []byte, 1)
	host, port := listen(t, func(c net.Conn) { data, _ := io.ReadAll(c); got <- data })
	result, _ := Send(Target{Connection: "network", Host: host, Port: port}, []byte("hola"))
	if result != Printed {
		t.Fatalf("esperaba printed, fue %s", result)
	}
	if string(<-got) != "hola" {
		t.Fatal("la impresora no recibió los datos")
	}
}

func TestNetworkClosedPortIsRejected(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // nadie escucha: conexión rechazada, ningún byte enviado
	result, detail := Send(Target{Connection: "network", Host: "127.0.0.1", Port: port}, []byte("x"))
	if result != Rejected {
		t.Fatalf("esperaba rejected, fue %s (%s)", result, detail)
	}
}

func TestNetworkStalledWriteIsAmbiguous(t *testing.T) {
	old := WriteTimeout
	WriteTimeout = 200 * time.Millisecond
	defer func() { WriteTimeout = old }()
	host, port := listen(t, func(c net.Conn) { time.Sleep(3 * time.Second); c.Close() }) // acepta y no lee
	big := make([]byte, 64<<20)                                                          // llena los búferes TCP
	result, detail := Send(Target{Connection: "network", Host: host, Port: port}, big)
	if result != Ambiguous {
		t.Fatalf("esperaba ambiguous, fue %s (%s)", result, detail)
	}
}

func TestUnknownConnectionAndMissingQueueAreRejected(t *testing.T) {
	if r, _ := Send(Target{Connection: "bluetooth"}, []byte("x")); r != Rejected {
		t.Fatal(r)
	}
	if r, _ := Send(Target{Connection: "system"}, []byte("x")); r != Rejected {
		t.Fatal(r)
	}
}

func TestTestTicketHasCutOnlyInEscpos(t *testing.T) {
	esc, plain := TestTicket(58, true), TestTicket(58, false)
	if esc[len(esc)-4] != 0x1d || plain[0] == 0x1b {
		t.Fatal("prueba mal formada")
	}
	for _, b := range plain {
		if b == 0x1b || b == 0x1d {
			t.Fatal("el texto plano no debe llevar comandos: " + strconv.Itoa(int(b)))
		}
	}
}
