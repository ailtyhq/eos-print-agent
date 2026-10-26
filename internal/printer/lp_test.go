//go:build darwin || linux

package printer

import "testing"

func TestClassifyLp(t *testing.T) {
	if r, _ := classifyLp("request id is EPSON-42 (1 file(s))\n", "EPSON"); r != Printed {
		t.Fatal(r)
	}
	if r, _ := classifyLp("la id de solicitud es EPSON-43\n", "EPSON"); r != Printed { // código de salida 0
		t.Fatal(r)
	}
}

func TestMissingQueueIsRejected(t *testing.T) {
	if r, detail := sendSystem("cola-que-no-existe-eos", []byte("x")); r != Rejected {
		t.Fatalf("esperaba rejected, fue %s (%s)", r, detail)
	}
}

func TestPausedQueueIsDetected(t *testing.T) {
	if !isDisabled("printer EPSON disabled since Tue Sep  2 14:02:10 2025 -\n\tPaused\n") {
		t.Fatal("pausada")
	}
	if !isDisabled("impresora GEZHI desactivada desde Tue Sep  2 14:02:10 2025:\n\tPaused\n") {
		t.Fatal("pausada (macOS en español)")
	}
	if isDisabled("la impresora EPSON está inactiva. activada desde Mon Oct 26 00:19:09 2026\n") {
		t.Fatal("activa (español)")
	}
	if isDisabled("printer EPSON is idle.  enabled since Mon Oct 26 00:33:29 2026\n") {
		t.Fatal("activa")
	}
}
