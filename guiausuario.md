# Guía de usuario de dtop

Esta guía explica para qué sirve `dtop`, cómo moverse por la aplicación y qué esperar de cada acción. No hace falta conocer el código para usarla.

## 1. Qué es dtop

`dtop` es una TUI para observar y operar un Docker Engine desde la terminal. Su objetivo es responder rápidamente:

- qué contenedores están corriendo;
- cuáles consumen más CPU o memoria;
- qué recursos Docker existen y cuáles están en uso;
- qué proyectos Docker Compose están activos;
- qué imágenes tienen una actualización disponible.

`dtop` no reemplaza Docker CLI ni edita archivos Compose. Presenta contexto y ejecuta acciones concretas usando Docker Engine o Docker CLI.

## 2. Iniciar y salir

Tras [instalar dtop](README.md), ejecútalo desde cualquier directorio:

```bash
dtop
```

Pulsa `q` para salir.

Requisitos actuales:

- Docker Engine local accesible;
- Docker CLI disponible en `PATH`;
- Buildx recomendado para comparar correctamente imágenes multi-arquitectura.

## 3. Navegación básica

| Tecla | Acción |
| --- | --- |
| `left` / `right` | Cambiar de vista |
| `up` / `down` o `j` / `k` | Mover la selección |
| `enter` | Abrir acciones o expandir un Stack según la vista |
| `d` | Abrir detalles del recurso enfocado cuando estén disponibles |
| `e` | Entrar o salir de selección múltiple |
| `space` | Marcar o desmarcar en selección múltiple |
| `a` | En edición, seleccionar o deseleccionar todas las casillas |
| `r` | Refrescar la vista activa |
| `x` | Abrir el menú global Advanced desde una vista principal |
| `?` | Abrir o cerrar Help |
| `Esc` | Volver, cancelar o cerrar el panel actual |
| `q` | Salir |

Las filas enfocadas se resaltan a ancho completo. En modo de selección múltiple aparece un checkbox por fila.

## 4. Vistas

### Containers

Muestra estado, salud, CPU, memoria, uptime e imagen de cada contenedor.

- `o` cambia el orden entre State, CPU, Memory y Name.
- `enter` abre Stop, Restart, Delete y Cancel para el contenedor enfocado.
- `d` abre detalles.
- `l` sigue los últimos logs.
- `s` abre `/bin/sh -l` dentro de un contenedor running.
- `e`, `space` y `enter` permiten Stop, Restart o Delete sobre uno o varios contenedores.

Una marca `U` junto al nombre indica una actualización disponible; `R` indica una descarga pendiente de aplicar. El menú contextual agrega `Pull update`, `Update now` o `Apply downloaded update` según el estado. En selección múltiple se procesan solo los contenedores elegibles y el menú informa los omitidos.

La shell usa el terminal real. Regresa a dtop con `Ctrl+D` o `exit`; mientras está abierta, `Esc` pertenece a la shell.

### Stacks

Agrupa contenedores por proyecto Docker Compose.

- `enter` expande o colapsa el stack.
- El panel inferior muestra directorio, archivos Compose y disponibilidad de acciones.
- `l` sobre el padre abre logs Compose.
- `l` o `s` sobre un hijo usa el contenedor enfocado.
- Las acciones Up, Stop, Restart y Down solo aparecen cuando existe metadata Compose local utilizable.

Down nunca agrega `--volumes`.

### Images

Muestra nombre, estado de update, tamaño, uso, edad e ID cuando el ancho lo permite.

`enter` abre las acciones de la imagen. `e`, `space` y `enter` aplican acciones en lote.

### Networks y Volumes

Muestran identidad, driver, uso y detalles del recurso.

- `enter` abre Delete o Cancel para el recurso enfocado.
- `d` abre detalles.
- `e`, `space` y `enter` permiten Delete en lote.
- Docker rechaza redes conectadas y volúmenes referenciados.
- Eliminar un volumen puede borrar datos persistentes.

## 5. Confirmaciones

Las acciones que cambian contenedores o recursos muestran una banda azul al pie de la aplicación con acción, destino y Engine.

```text
CONFIRM: Recreate containers
Target: example:latest on LOCAL desktop-linux
Are you sure? [y/N] | [Esc] cancel
```

- `y` confirma;
- `n` o `Esc` cancela;
- la confirmación permanece visible hasta decidir.

`Pull update` no pide confirmación porque solo descarga una imagen. No reinicia ni reemplaza contenedores.

## 6. Limpieza avanzada

Pulsa `x` desde cualquiera de las cinco vistas principales para abrir Advanced. No se abre encima de Help, detalles, logs, selección múltiple ni menús de acciones.

El menú ofrece estas operaciones locales y muestra el comando exacto de la opción enfocada:

| Opción | Comando |
| --- | --- |
| Delete stopped containers | `docker container prune --force` |
| Delete unused images | `docker image prune --all --force` |
| Delete unused networks | `docker network prune --force` |
| Delete unused volumes | `docker volume prune --force` |
| Delete unused Docker data | `docker system prune --all --force` |

Antes de ejecutar hay que escribir exactamente `prune` y pulsar `Enter`. `Esc` cancela. El prune de sistema no incluye `--volumes`; los volúmenes se limpian solo con la opción separada. Al terminar, el resultado permanece visible y dtop recarga Containers, Images, Networks y Volumes. Los endpoints remotos siguen rechazados.

## 7. Updates de imágenes

La columna UPDATE usa estos indicadores:

| Estado | Significado |
| --- | --- |
| `U` | Existe un digest remoto distinto; hay update disponible |
| `R` | La imagen nueva ya fue descargada y el contenedor necesita recreate |
| `=` | La referencia local y remota coinciden |
| `P` | La referencia está fijada por digest |
| `?` | No fue posible comparar de forma segura |
| `...` | La consulta está en curso |

El flujo normal para un contenedor directo es:

```text
U -> Pull update -> R -> Recreate containers -> =
```

1. En una fila `U`, pulsa `enter` y elige `Pull update`.
2. Pull comienza de inmediato.
3. La fila pasa a `R` cuando el contenedor sigue usando el image ID anterior.
4. Pulsa `enter`, elige `Recreate containers` y confirma con `y`.
5. dtop reconstruye el contenedor con la imagen descargada y vuelve a comprobar el digest.

La recreación directa conserva la configuración inspeccionada del contenedor. Si el destino pertenece a Compose, dtop deduplica los hijos por Stack y servicio y valida la imagen contra la configuración Compose efectiva. Antes del pull guarda un marcador seguro y después persiste el digest descargado en `$XDG_STATE_HOME/dtop/compose-updates.json`, con fallback `~/.local/state/dtop/compose-updates.json`.

Un Pull de Compose puede sobrevivir al reinicio de dtop y a `docker compose down`. Mientras el digest descargado sea distinto del aplicado, un Stack Down o Stopped muestra `Apply downloaded update` en lugar de Up. Apply requiere `y`, vuelve a validar configuración e imagen local, ejecuta `up -d` y solo marca el digest como aplicado después de inspeccionar los contenedores resultantes. Si el estado no puede leerse o verificarse, Up falla de forma segura. Servicios build-managed, pinned, one-off o con una política de pull no determinista no se presentan como actualizables.

## 8. Docker Hub y el estado `?`

Docker Hub limita consultas anónimas. Si la cuota se agota, dtop muestra `?` en lugar de afirmar un estado incorrecto.

Inicia sesión una vez:

```bash
docker login
```

dtop reutiliza la configuración y los credential helpers de Docker CLI. No almacena ni muestra tokens.

Help muestra este recordatorio mientras no detecte una credencial Docker Hub configurada:

```text
Image updates
To get updates for your container images, log in to Docker Hub with: docker login
```

La comprobación se repite al abrir Help, por lo que el mensaje puede desaparecer sin reiniciar dtop después de iniciar sesión.

El mismo aviso aparece en la pantalla principal, justo encima de la línea de ayudas inferior, hasta detectar una sesión utilizable.

## 9. Configuración opcional

Rutas, en orden de precedencia:

1. `/etc/dtop/dtop.conf`;
2. `$XDG_CONFIG_HOME/dtop/dtop.conf`;
3. `~/.config/dtop/dtop.conf`.

Ejemplo:

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

[compose "nextcloud"]
working_dir = /srv/docker/nextcloud
files = compose.yaml, compose.prod.yaml
```

`memory_mode` acepta `usage`, `percent` o `both`. Los colores aceptan valores ANSI/256 de `0` a `255`.

## 10. Errores y datos parciales

- Containers se refresca cada dos segundos.
- Images, Networks y Volumes se reconcilian cada cinco segundos.
- Si una fuente falla, dtop conserva sus últimas filas y las marca como datos conocidos pero no actuales.
- Un fallo de registry produce `?`, no un falso `=`.
- Los resultados de acciones muestran cuántos objetivos terminaron y cuántos fallaron.

## 11. Limitaciones actuales

- Un solo Docker Engine local por ejecución.
- El update Compose requiere metadata local válida del Stack.
- Historial de versiones y rollback todavía no están implementados.
- Los bind mounts de un Compose después de Down todavía no se persisten.
- Docker Hub puede limitar consultas sin login.
