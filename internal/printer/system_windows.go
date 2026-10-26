//go:build windows

package printer

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winspool         = windows.NewLazySystemDLL("winspool.drv")
	procOpenPrinter  = winspool.NewProc("OpenPrinterW")
	procClosePrinter = winspool.NewProc("ClosePrinter")
	procStartDoc     = winspool.NewProc("StartDocPrinterW")
	procEndDoc       = winspool.NewProc("EndDocPrinter")
	procStartPage    = winspool.NewProc("StartPagePrinter")
	procEndPage      = winspool.NewProc("EndPagePrinter")
	procWritePrinter = winspool.NewProc("WritePrinter")
	procEnumPrinters = winspool.NewProc("EnumPrintersW")
)

type docInfo1 struct {
	DocName    *uint16
	OutputFile *uint16
	Datatype   *uint16
}

// sendSystem envía al spooler de Windows como trabajo RAW. Los fallos antes de escribir (la impresora no existe o no
// acepta el documento) son Rejected; un fallo al escribir o al cerrar el documento, Ambiguous.
func sendSystem(name string, data []byte) (Result, string) {
	pname, _ := syscall.UTF16PtrFromString(name)
	var handle syscall.Handle
	if r, _, err := procOpenPrinter.Call(uintptr(unsafe.Pointer(pname)), uintptr(unsafe.Pointer(&handle)), 0); r == 0 {
		return Rejected, fmt.Sprintf("No se encontró la impresora %q: %v", name, err)
	}
	defer procClosePrinter.Call(uintptr(handle))
	doc, _ := syscall.UTF16PtrFromString("Comanda EOS")
	raw, _ := syscall.UTF16PtrFromString("RAW")
	info := docInfo1{DocName: doc, Datatype: raw}
	if r, _, err := procStartDoc.Call(uintptr(handle), 1, uintptr(unsafe.Pointer(&info))); r == 0 {
		return Rejected, fmt.Sprintf("La impresora %q no aceptó el documento RAW: %v", name, err)
	}
	procStartPage.Call(uintptr(handle))
	var written uint32
	r, _, err := procWritePrinter.Call(uintptr(handle), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&written)))
	procEndPage.Call(uintptr(handle))
	end, _, _ := procEndDoc.Call(uintptr(handle))
	if r == 0 || int(written) != len(data) {
		return Ambiguous, fmt.Sprintf("Se cortó el envío a %q tras %d de %d bytes (%v): puede que se haya impreso.", name, written, len(data), err)
	}
	if end == 0 {
		return Ambiguous, fmt.Sprintf("La impresora %q no confirmó el fin del documento.", name)
	}
	return Printed, ""
}

type printerInfo4 struct {
	PrinterName *uint16
	ServerName  *uint16
	Attributes  uint32
}

// SystemPrinters lista las impresoras locales y conectadas (EnumPrinters, nivel 4).
func SystemPrinters() []string {
	const flags = 0x2 | 0x4 // PRINTER_ENUM_LOCAL | PRINTER_ENUM_CONNECTIONS
	var needed, returned uint32
	procEnumPrinters.Call(flags, 0, 4, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if needed == 0 {
		return nil
	}
	buf := make([]byte, needed)
	if r, _, _ := procEnumPrinters.Call(flags, 0, 4, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed),
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned))); r == 0 {
		return nil
	}
	infos := unsafe.Slice((*printerInfo4)(unsafe.Pointer(&buf[0])), returned)
	names := make([]string, 0, returned)
	for _, info := range infos {
		names = append(names, windows.UTF16PtrToString(info.PrinterName))
	}
	return names
}
