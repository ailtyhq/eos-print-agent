# EOS Print Agent

Agente de impresión de comandas de EOS (HU 20.8d). Se instala en un equipo del local, se vincula a una sucursal con un
código de un solo uso y envía cada tirilla a su impresora: por red (TCP, puerto 9100) o a una impresora del sistema (USB).

- macOS (Apple Silicon e Intel): LaunchAgent del usuario; impresoras USB por CUPS (`lp -o raw`).
- Windows: servicio con la cuenta LocalService; impresoras por el spooler (trabajo RAW).
- Linux: servicio de systemd con el usuario `eos-print` (grupo `lp`); impresoras por CUPS.

```sh
eos-print-agent pair --url https://<tu-negocio> --code ABCD-1234 --name "Mac de cocina"
eos-print-agent install        # en Linux con sudo; en Windows desde una consola de administrador
eos-print-agent status | logs | stop | start | uninstall
eos-print-agent test --host 192.168.1.50   # o --queue <cola>: prueba local sin EOS
```

Resultados que confirma a EOS: `printed` (la cola o la impresora aceptó los datos; no es confirmación física),
`rejected` (no se aceptó nada: EOS puede reintentar solo) y `ambiguous` (pudo imprimirse: EOS no lo reenvía solo).
El token se guarda con permisos de solo dueño y nunca se escribe completo en el log. No se actualiza solo.

Compatibilidad: protocolo 1 (backend EOS con la HU 20.8d). Releases: `./build-release.sh <versión>`, que publica
`SHA256SUMS`. Los binarios aún no están firmados (deuda técnica): ver la guía de instalación en EOS.
