package printer

import (
	"bytes"
	"strings"
)

// TestTicket es una tirilla local de prueba (antes de vincular), en ESC/POS con CP858 o en texto plano.
func TestTicket(width int, escpos bool) []byte {
	cols := 48
	if width == 58 {
		cols = 32
	}
	var b bytes.Buffer
	if escpos {
		// FS . apaga el modo chino de fábrica de muchas térmicas (si no, las tildes salen como caracteres chinos).
		b.Write([]byte{0x1b, '@', 0x1c, '.', 0x1b, 't', 19, 0x1b, 'a', 1, 0x1b, 'E', 1})
	}
	b.WriteString("EOS - PRUEBA DE IMPRESION\n")
	if escpos {
		b.Write([]byte{0x1b, 'E', 0, 0x1b, 'a', 0})
	}
	b.WriteString(strings.Repeat("-", cols) + "\n")
	b.Write(cp858("Acentos: áéíóú ñ Ñ ¿? ¡!\n"))
	b.WriteString("Si ves esta tirilla, la impresora\nesta lista para el agente.\n\n\n\n")
	if escpos {
		b.Write([]byte{0x1d, 'V', 0x42, 0})
	}
	return b.Bytes()
}

// cp858 codifica los caracteres del español que usa la prueba (el resto de tirillas las genera EOS).
func cp858(s string) []byte {
	table := map[rune]byte{'á': 0xa0, 'é': 0x82, 'í': 0xa1, 'ó': 0xa2, 'ú': 0xa3, 'ñ': 0xa4, 'Ñ': 0xa5, '¿': 0xa8, '¡': 0xad}
	var out []byte
	for _, r := range s {
		if c, ok := table[r]; ok {
			out = append(out, c)
		} else if r < 128 {
			out = append(out, byte(r))
		} else {
			out = append(out, '?')
		}
	}
	return out
}
