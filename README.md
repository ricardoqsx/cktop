# cktop

`cktop` es el repositorio de desarrollo conjunto de dos aplicaciones TUI para operación cloud-native:

- **dtop:** monitorización y operaciones contextuales para Docker y Docker Compose.
- **ktop:** resumen operacional y diagnóstico inicial de clústeres Kubernetes.

Ambas aplicaciones se construirán en Go sobre una base visual común, manteniendo separados sus dominios, dependencias y ciclos de entrega. El repositorio está diseñado para que `dtop`, `ktop` y la librería TUI compartida puedan convertirse en proyectos independientes cuando alcancen la madurez necesaria.

El objetivo no es reemplazar Docker CLI, `kubectl` o herramientas de administración completas. La prioridad es abrir una aplicación rápida, entender el estado actual y llegar a la acción o investigación relevante con pocas interacciones.

## Estado

El workspace y la base TUI inicial están preparados. `dtop` detecta un Docker Engine local y ofrece vistas responsive de Containers, Stacks, Images, Networks y Volumes. Las filas enfocadas se resaltan a ancho completo y los colores de acento/foco son configurables por aplicación. Las mutaciones muestran una confirmación contextual sin ocultar la tabla y usan `y/N`; Pull de imágenes se ejecuta directamente porque no recrea ni reinicia contenedores. Tras ejecutar, la tabla vuelve a estar activa, la selección se limpia y un resultado temporal informa éxito parcial o total. Down no elimina volúmenes. Si faltan metadatos Compose, el stack sigue siendo observable pero no expone acciones. En D2, Images, Networks y Volumes admiten selección múltiple y eliminación secuencial no forzada. Docker rechaza redes conectadas y volúmenes referenciados; eliminar un volumen puede borrar datos persistentes. Sus referencias se calculan desde los contenedores listados. Posteriormente se añadirá un único host LAN o remoto mediante SSH o TLS.

En Stacks, el panel del proyecto seleccionado permanece debajo de la tabla con directorio, archivos Compose y disponibilidad de acciones. `l` en el padre sigue los últimos 100 logs Compose; `l` en un hijo abre sus logs de contenedor. `s` en un hijo enfocado abre una shell local real mediante `docker exec -it <container-id> /bin/sh -l`; volver con `Ctrl+D` o `exit`. Durante esa shell, el terminal pertenece al proceso y `Esc` no vuelve a dtop. `Esc` cancela ambos streams de logs y vuelve a Stacks. `e` limita la selección múltiple de Restart/Stop a los hijos. Containers se actualiza cada dos segundos; Images, Networks y Volumes se reconcilian en segundo plano cada cinco segundos. Si falla una fuente individual, su pestaña conserva los últimos datos conocidos y los marca como parciales.

La próxima ampliación de Stacks conservará bind mounts observados en un archivo de estado separado de `dtop.conf`, para que sigan visibles después de `docker compose down`. Todavía no está implementada.

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

Los binarios generados por `go build` quedan en la raíz si no se indica una ruta de salida. Son artefactos locales y no deben versionarse.

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

La detección de actualizaciones consulta cada referencia única usada por contenedores `running` mediante Docker CLI, reutilizando sus credenciales y credential helpers. Prefiere `docker buildx imagetools inspect` para obtener el digest del índice y usa `docker manifest inspect --verbose` como fallback verificable. Compara el digest remoto con los `RepoDigests` locales; `U` indica una actualización, `R` una imagen descargada pendiente de aplicar, `=` actual, `P` una referencia fijada por digest, `?` una comparación no verificable y `...` una consulta activa. No filtra `latest`: una etiqueta versionada también se consulta. Images conserva la vista de referencia, mientras Containers muestra una marca compacta `U` o `R` junto al nombre y expone `Pull update`, `Update now` o `Apply downloaded update` desde su menú contextual. La selección múltiple procesa únicamente destinos elegibles y muestra cuántos se omiten. Para contenedores directos, `Update now` hace pull y recrea conservando la configuración inspeccionada; los contenedores `AutoRemove` se rechazan porque no pueden restaurarse con seguridad. En Compose, una acción iniciada desde hijos se deduplica por Stack y servicio; una selección de padres opera el Stack completo. Ambas rutas requieren metadata válida y ejecutan `docker compose pull` seguido de `up -d`; la ruta por servicio añade `--no-deps` para respetar el alcance confirmado. Pull no reinicia ni recrea contenedores. Si Docker Hub no tiene una credencial configurada, Help muestra el recordatorio persistente `docker login` hasta detectarla.

Para registrar opcionalmente un proyecto Compose local:

```ini
[compose "nextcloud"]
working_dir = /srv/docker/nextcloud
files = compose.yaml, compose.prod.yaml
```

El nombre, `working_dir` y `files` son obligatorios y no pueden estar vacíos. `working_dir` debe existir y ser un directorio. Cada entrada de `files` se separa por coma; las rutas relativas se resuelven contra `working_dir`, y las absolutas se conservan. dtop solo comprueba que las rutas existan y no lee su contenido. Una inscripción válida prevalece sobre los metadatos detectados por labels. Una inscripción válida sin contenedores se muestra como `Down`; no se infiere `Never deployed`. Un archivo ausente se muestra como `Missing compose file`. Inscripciones malformadas no crean stacks y aparecen como diagnósticos controlados en Stacks. En Stacks, `e`, espacio y Enter permiten seleccionar proyectos. Down/Missing con metadatos válidos expone Up; Running/Mixed expone Stop, Restart y Down; Stopped expone Up, Restart y Down. Cada acción pide `Are you sure? [y/N]`. dtop invoca localmente `docker compose` con el nombre, directorio y cada archivo explícitos; nunca añade `--volumes`.

Configuración global:

```text
/etc/dtop/dtop.conf
```

La configuración de usuario la sobrescribe y se busca en:

```text
$XDG_CONFIG_HOME/dtop/dtop.conf
```

Si `XDG_CONFIG_HOME` no está definido se usa `~/.config/dtop/dtop.conf`.

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
