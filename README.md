# cktop

`cktop` es el repositorio de desarrollo conjunto de dos aplicaciones TUI para operación cloud-native:

- **dtop:** monitorización y operaciones contextuales para Docker y Docker Compose.
- **ktop:** resumen operacional y diagnóstico inicial de clústeres Kubernetes.

Ambas aplicaciones se construirán en Go sobre una base visual común, manteniendo separados sus dominios, dependencias y ciclos de entrega. El repositorio está diseñado para que `dtop`, `ktop` y la librería TUI compartida puedan convertirse en proyectos independientes cuando alcancen la madurez necesaria.

El objetivo no es reemplazar Docker CLI, `kubectl` o herramientas de administración completas. La prioridad es abrir una aplicación rápida, entender el estado actual y llegar a la acción o investigación relevante con pocas interacciones.

## Estado

El workspace y la base TUI inicial están preparados. `dtop` detecta un Docker Engine local y ofrece vistas responsive de Containers, Stacks, Images, Networks y Volumes. Stacks descubre proyectos Compose etiquetados sin requerir registro, agrega CPU y memoria de sus contenedores running y permite `Up`, `Stop`, `Restart` y `Down` locales cuando el estado y los metadatos explícitos lo permiten. Las mutaciones requieren confirmación `Are you sure? [y/N]`; Down no elimina volúmenes. Si faltan metadatos Compose, el stack sigue siendo observable pero no expone acciones. En D2, Images, Networks y Volumes admiten selección múltiple y eliminación secuencial no forzada con confirmación explícita. Docker rechaza redes conectadas y volúmenes referenciados; eliminar un volumen puede borrar datos persistentes. Sus referencias se calculan desde los contenedores listados. Posteriormente se añadirá un único host LAN o remoto mediante SSH o TLS.

En Stacks, el panel del proyecto seleccionado permanece debajo de la tabla con directorio, archivos Compose y disponibilidad de acciones. `l` en el padre sigue los últimos 100 logs Compose; `l` en un hijo abre sus logs de contenedor. `s` en un hijo enfocado abre una shell local real mediante `docker exec -it <container-id> /bin/sh -l`; volver con `Ctrl+D` o `exit`. Durante esa shell, el terminal pertenece al proceso y `Esc` no vuelve a dtop. `Esc` cancela ambos streams de logs y vuelve a Stacks. `e` limita la selección múltiple de Restart/Stop a los hijos.

## Desarrollo local

Ejecutar `dtop`:

```bash
go run ./apps/dtop/cmd/dtop
```

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
enter           open details for the current resource, expand a stack, or open contextual stack/container actions
l               follow the last 100 log lines for the current container, focused stack child, or stack parent
s               open a local interactive shell for the selected container or focused expanded stack child; use Ctrl+D or exit to return
esc             stop logs, return from details, or collapse a stack
e               enter/exit multi-selection mode in Containers, Stacks, Images, Networks or Volumes; on a stack child, select children of that stack
space           toggle the current resource in multi-selection mode
enter           open selected resource actions, or details when not editing
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
```

Valores disponibles: `usage`, `percent` y `both`.

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

## Licencia

Distribuido bajo la [licencia MIT](LICENSE).
