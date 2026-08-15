# cktop

`cktop` es el repositorio de desarrollo conjunto de dos aplicaciones TUI para operación cloud-native:

- **dtop:** monitorización y operaciones contextuales para Docker y Docker Compose.
- **ktop:** resumen operacional y diagnóstico inicial de clústeres Kubernetes.

Ambas aplicaciones se construirán en Go sobre una base visual común, manteniendo separados sus dominios, dependencias y ciclos de entrega. El repositorio está diseñado para que `dtop`, `ktop` y la librería TUI compartida puedan convertirse en proyectos independientes cuando alcancen la madurez necesaria.

El objetivo no es reemplazar Docker CLI, `kubectl` o herramientas de administración completas. La prioridad es abrir una aplicación rápida, entender el estado actual y llegar a la acción o investigación relevante con pocas interacciones.

## Estado

El workspace y la base TUI inicial están preparados. `dtop` detecta un Docker Engine local y ofrece vistas responsive de Containers, Stacks, Images, Networks y Volumes. Las filas enfocadas se resaltan a ancho completo y los colores de acento/foco son configurables por aplicación. Las mutaciones muestran una confirmación contextual sin ocultar la tabla y usan `y/N`; Pull de imágenes se ejecuta directamente porque no recrea ni reinicia contenedores. Tras ejecutar, la tabla vuelve a estar activa, la selección se limpia y un resultado temporal informa éxito parcial o total. Down no elimina volúmenes. Si faltan metadatos Compose, el stack sigue siendo observable pero no expone acciones. En D2, Images, Networks y Volumes admiten selección múltiple y eliminación secuencial no forzada. Docker rechaza redes conectadas y volúmenes referenciados; eliminar un volumen puede borrar datos persistentes. Sus referencias se calculan desde los contenedores listados. La primera versión se limita deliberadamente a Docker local.

En Stacks, el panel del proyecto seleccionado permanece debajo de la tabla con directorio, archivos Compose y disponibilidad de acciones. `l` en el padre sigue los últimos 100 logs Compose; `l` en un hijo abre sus logs de contenedor. `s` en un hijo enfocado abre una shell local real mediante `docker exec -it <container-id> /bin/sh -l`; volver con `Ctrl+D` o `exit`. Durante esa shell, el terminal pertenece al proceso y `Esc` no vuelve a dtop. `Esc` cancela ambos streams de logs y vuelve a Stacks. `e` limita la selección múltiple de Restart/Stop a los hijos. Containers se actualiza cada dos segundos; Images, Networks y Volumes se reconcilian en segundo plano cada cinco segundos. Si falla una fuente individual, su pestaña conserva los últimos datos conocidos y los marca como parciales.

Desde cualquier vista principal, `[x]` abre Advanced para limpiar contenedores detenidos, imagenes, redes, volumenes o datos Docker sin uso. La opcion enfocada muestra el comando exacto y exige escribir `prune` antes de ejecutarlo. Estas operaciones solo se admiten sobre el Engine local. `docker system prune --all --force` no incluye volumenes; su limpieza permanece separada mediante `docker volume prune --force`. Al terminar, dtop conserva la salida o el error y recarga Containers, Images, Networks y Volumes.

La próxima ampliación de Stacks conservará bind mounts observados en un archivo de estado separado de `dtop.conf`, para que sigan visibles después de `docker compose down`. Todavía no está implementada.

## Plataformas soportadas

La distribución de `dtop` se prepara para:

- Linux AMD64 y ARM64 con Docker Engine local.
- WSL mediante el mismo binario Linux y un Engine accesible desde la distribución.
- macOS Intel y Apple Silicon con Docker Desktop.

Windows nativo, ejecutables `.exe`, PowerShell y conexión mediante Docker named pipe no forman parte del alcance. Docker remoto por SSH o TLS tampoco se anuncia como compatible en esta versión.

## Release dtop v0.4.1

La release estable local-only recomendada se distribuye mediante un instalador fijado al tag versionado `dtop-v0.4.1`, que no debe moverse ni reemplazarse después de publicar. `v0.4.1` reemplaza a `v0.4.0` porque corrige las rutas del instalador y la clave pública dentro de `SHA256SUMS`; los binarios de aplicación mantienen el mismo alcance funcional.

```bash
curl -fsSL https://github.com/ricardoqsx/cktop/releases/download/dtop-v0.4.1/install-dtop.sh | sh
```

Alternativa con `wget`:

```bash
wget -qO- https://github.com/ricardoqsx/cktop/releases/download/dtop-v0.4.1/install-dtop.sh | sh
```

El instalador:

- detecta Linux/macOS y AMD64/ARM64;
- pregunta si la instalación será para el usuario actual o para todo el sistema;
- permite confirmar o cambiar el directorio del binario;
- muestra las rutas finales antes de continuar;
- verifica el SHA-256 embebido del asset y, si Minisign está instalado, también la firma publicada;
- instala `dtop` y comprueba `dtop --version` antes de copiarlo;
- crea una configuración completa y su `.example`, sin sobrescribir una configuración existente;
- solicita `sudo` únicamente al escribir una instalación de sistema.

Para CI o instalación no interactiva del usuario actual:

```bash
curl -fsSL https://github.com/ricardoqsx/cktop/releases/download/dtop-v0.4.1/install-dtop.sh | sh -s -- --yes --scope user
```

Para instalar otra versión, se reemplaza `0.4.1` en la URL por la versión publicada correspondiente. Cada instalador contiene los hashes exactos de sus propios assets; no se debe usar una URL `latest`.

Rutas predeterminadas:

| Alcance | Binario | Configuración |
| --- | --- | --- |
| Usuario | `~/.local/bin/dtop` | `${XDG_CONFIG_HOME:-~/.config}/dtop/dtop.conf` |
| Sistema | `/usr/local/bin/dtop` | `/etc/dtop/dtop.conf` |

Una actualización se realiza ejecutando el instalador de la nueva versión. El binario se reemplaza y `dtop.conf` se conserva; la configuración de referencia se actualiza en `dtop.conf.example`.

Los assets manuales, `SHA256SUMS`, `SHA256SUMS.minisig`, la clave pública `dtop.minisign.pub` y la procedencia verificable se publican en [GitHub Releases](https://github.com/ricardoqsx/cktop/releases). Verificación manual:

```bash
minisign -Vm SHA256SUMS -x SHA256SUMS.minisig -p dtop.minisign.pub
asset=dtop_0.4.1_darwin_arm64.tar.gz
grep " $asset$" SHA256SUMS | shasum -a 256 -c -
```

Antes de confiar en la firma, la clave pública descargada debe coincidir con `configs/dtop.minisign.pub` en el tag fuente de esa versión. En Linux puede sustituirse `shasum -a 256` por `sha256sum`.

Para desinstalar una instalación de usuario:

```bash
rm "$HOME/.local/bin/dtop"
rm -rf "${XDG_CONFIG_HOME:-$HOME/.config}/dtop"
```

La desinstalación de sistema usa las mismas rutas con `sudo`; debe conservarse `/etc/dtop/dtop.conf` si se quiere reutilizar la configuración.

## Desarrollo local

Ejecutar `dtop`:

```bash
go run ./apps/dtop/cmd/dtop
```

La interfaz de `dtop` usa el locale del entorno y admite ingles (`en`) y espanol (`es`). Puede forzarse por ejecucion:

```bash
DTOP_LOCALE=es go run ./apps/dtop/cmd/dtop
DTOP_LOCALE=en go run ./apps/dtop/cmd/dtop
```

La precedencia es `DTOP_LOCALE`, `LC_ALL`, `LC_MESSAGES` y `LANG`. Las variantes como `es_ES.UTF-8` se reducen al idioma base; locales no admitidos y mensajes ausentes usan ingles.

Ejecutar `ktop`:

```bash
go run ./apps/ktop/cmd/ktop
```

Salir de cualquiera de las dos TUI:

```text
q
```

Controles iniciales de `dtop`:

```text
left/right      switch Containers, Stacks, Images, Networks and Volumes
up/down or j/k  select the current resource
x               open the global Advanced cleanup menu
o               cycle State, CPU, Memory and Name sorting
r               refresh the active resource list
enter           open contextual actions for Containers and Images, details for Networks/Volumes, or expand a Stack
d               open details for the current Container
l               follow the last 100 log lines for the current container, focused stack child, or stack parent
s               open a local interactive shell for the selected container or focused expanded stack child; use Ctrl+D or exit to return
esc             stop logs, return from details, or collapse a stack
e               enter/exit multi-selection mode in Containers, Stacks, Images, Networks or Volumes; on a stack child, select children of that stack
space           toggle the current resource in multi-selection mode
a               in multi-selection mode, select or clear all checkboxes
?               help
q               quit
```

Validar el workspace:

```bash
gofmt -l apps libs
go test ./apps/dtop/... ./apps/ktop/... ./libs/tui/...
go vet ./apps/dtop/... ./apps/ktop/... ./libs/tui/...
go build ./apps/dtop/cmd/dtop
go build ./apps/ktop/cmd/ktop
```

Las pruebas de adaptador que requieren un Docker Engine local son opt-in y no realizan mutaciones:

```bash
DTOP_INTEGRATION=1 go test ./apps/dtop/internal/adapters/docker -run 'TestRuntime(NetworkAndVolume|Images|Snapshot|Stacks)Integration'
```

La integracion D4 mutativa solo debe ejecutarse contra un Engine desechable y exige confirmar su ID explícitamente:

```bash
DTOP_MUTATION_INTEGRATION=1 \
DTOP_MUTATION_ENGINE_ID="$(docker info --format '{{.ID}}')" \
go test ./apps/dtop/internal/application -run TestComposeUpdatePersistenceMutationIntegration -count=1
```

Esta prueba levanta un proyecto temporal haciendo que `alpine:3.20` use inicialmente el image ID de `3.19`, ejecuta el ciclo D4 contra el `3.20` remoto real y restaura cualquier tag local previo durante el cleanup.

Los binarios generados por `go build` quedan en la raíz si no se indica una ruta de salida. Son artefactos locales y no deben versionarse.

## CLI de dtop

```text
dtop [--config PATH] [--version]
```

- `--help` muestra el contrato del comando sin cargar configuración ni conectar con Docker.
- `--version` imprime versión, commit y fecha de build sin iniciar la TUI.
- `--config PATH` carga un archivo adicional con la prioridad más alta. La ruta debe existir y ser válida.

Los builds de desarrollo muestran `dev`, `unknown` y `unknown`. El workflow de release inyectará los valores mediante linker flags:

```bash
go build -ldflags "-X main.version=0.4.0 -X main.commit=$(git rev-parse --short HEAD) -X main.buildDate=2026-08-15T00:00:00Z" -o dtop ./apps/dtop/cmd/dtop
./dtop --version
```

## Configuración de dtop

`dtop` funciona sin configuración. Para elegir cómo mostrar memoria puede crearse:

```ini
[display]
memory_mode = both
accent_color = 63
focus_color = 15

[updates]
enabled = true
scope = running
interval = 15m
concurrency = 4
```

Valores disponibles: `usage`, `percent` y `both`.
`accent_color` y `focus_color` aceptan valores ANSI/256 de `0` a `255`; sus defaults son `63` y `15`. El acento es el fondo del foco en tablas y menús, mientras `focus_color` define el texto resaltado.

La detección de actualizaciones consulta cada referencia única usada por contenedores `running` mediante Docker CLI, reutilizando sus credenciales y credential helpers. Prefiere `docker buildx imagetools inspect` para obtener el digest del índice y usa `docker manifest inspect --verbose` como fallback verificable. Compara el digest remoto con los `RepoDigests` locales; `U` indica una actualización, `R` una imagen descargada pendiente de aplicar, `=` actual, `P` una referencia fijada por digest, `?` una comparación no verificable y `...` una consulta activa. No filtra `latest`: una etiqueta versionada también se consulta. Images conserva la vista de referencia, mientras Containers muestra una marca compacta `U` o `R` junto al nombre y expone `Pull update`, `Update now` o `Apply downloaded update` desde su menú contextual. La selección múltiple procesa únicamente destinos elegibles y muestra cuántos se omiten. Para contenedores directos, `Update now` hace pull y recrea conservando la configuración inspeccionada; los contenedores `AutoRemove` se rechazan porque no pueden restaurarse con seguridad. En Compose registrado, dtop correlaciona cada servicio con `docker compose config`, escribe un marcador durable antes del pull y persiste los digests descargado/aplicado en `$XDG_STATE_HOME/dtop/compose-updates.json`. Una acción iniciada desde hijos se deduplica por Stack y servicio; una selección de padres opera los servicios elegibles del Stack. `Up` queda bloqueado mientras exista una descarga pendiente y la TUI ofrece `Apply downloaded update`; `system down` no borra esa evidencia. Pull no reinicia ni recrea contenedores. Si Docker Hub no tiene una credencial configurada, Help muestra el recordatorio persistente `docker login` hasta detectarla.

Para registrar opcionalmente un proyecto Compose local:

```ini
[compose "nextcloud"]
working_dir = /srv/docker/nextcloud
files = compose.yaml, compose.prod.yaml
```

El nombre, `working_dir` y `files` son obligatorios y no pueden estar vacíos. `working_dir` debe existir y ser un directorio. Cada entrada de `files` se separa por coma; las rutas relativas se resuelven contra `working_dir`, y las absolutas se conservan. dtop solo comprueba que las rutas existan y no lee su contenido. Una inscripción válida prevalece sobre los metadatos detectados por labels. Una inscripción válida sin contenedores se muestra como `Down`; no se infiere `Never deployed`. Un archivo ausente se muestra como `Missing compose file`. Inscripciones malformadas no crean stacks y aparecen como diagnósticos controlados en Stacks. En Stacks, `e`, espacio y Enter permiten seleccionar proyectos. Down con metadatos válidos expone Up, salvo que exista una actualización descargada pendiente, en cuyo caso expone Apply; Running/Mixed expone Stop, Restart y Down; Stopped aplica la misma protección antes de Up. Cada mutación pide `Are you sure? [y/N]`. dtop invoca localmente `docker compose` con el nombre, directorio y cada archivo explícitos; nunca añade `--volumes`.

Configuración global:

```text
/etc/dtop/dtop.conf
```

La configuración de usuario la sobrescribe y se busca en:

```text
$XDG_CONFIG_HOME/dtop/dtop.conf
```

Si `XDG_CONFIG_HOME` no está definido se usa `~/.config/dtop/dtop.conf`.

La precedencia completa, de menor a mayor prioridad, es:

1. Valores internos seguros.
2. `/etc/dtop/dtop.conf` si existe.
3. `$XDG_CONFIG_HOME/dtop/dtop.conf` o `~/.config/dtop/dtop.conf` si existe.
4. El archivo indicado por `DTOP_CONFIG`.
5. El archivo indicado por `--config PATH`.

`DTOP_CONFIG` y `--config` se superponen a las fuentes anteriores, por lo que pueden modificar solo las claves necesarias. Cuando cualquiera de esas rutas se declara explícitamente, dtop termina con un error controlado si el archivo no existe, no puede leerse o contiene una opción inválida.

La plantilla completa distribuida por el instalador también está disponible en [`configs/dtop.conf.example`](configs/dtop.conf.example).

## Estructura inicial

```text
apps/dtop    aplicación Docker
apps/ktop    aplicación Kubernetes
libs/tui     base visual compartida
```

## Documentación

La planificación, arquitectura y visiones de producto viven localmente en `.proyects/`. Esa carpeta no forma parte del repositorio público.

- [Guia de usuario de dtop](.proyects/userguide.md): uso diario, vistas, acciones, updates y configuracion.
- [Documentacion tecnica de dtop](.proyects/documentacion.md): mapa del codigo, flujos y guia de mantenimiento.
- [Plan](.proyects/PLAN.md): estado de hitos y siguientes entregas.
- [Arquitectura](.proyects/ARCHITECTURE.md) y [decisiones](.proyects/DECISIONS.md): limites y reglas duraderas.

## Licencia

Distribuido bajo la [licencia MIT](LICENSE).
