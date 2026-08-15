# cktop

**cktop** is the development home of two terminal interfaces for cloud-native operations:

- **dtop** — observe and operate a local Docker Engine and registered Docker Compose projects.
- **ktop** — cluster status and initial diagnosis for Kubernetes (in development).

The goal is not to replace `docker`, `kubectl` or full management tools. The priority is to open a fast application, understand the current state, and reach the relevant action or investigation in a few keystrokes.

---

## English

### What is dtop?

`dtop` is a fast, keyboard-driven terminal UI that shows what is happening on your Docker host and lets you act on it: containers, CPU and memory, running stacks, images, networks, volumes, and available image updates — all in one screen, with no daemon and no web service.

It works **without configuration** and never replaces the Docker CLI. It gives you context and performs concrete actions through your local Docker Engine.

### Installation

Install the latest stable release with a single command:

```bash
curl -fsSL https://github.com/ricardoqsx/cktop/releases/download/dtop-v0.4.1/install-dtop.sh | sh
```

Or with `wget`:

```bash
wget -qO- https://github.com/ricardoqsx/cktop/releases/download/dtop-v0.4.1/install-dtop.sh | sh
```

For CI or a non-interactive install of the current user:

```bash
curl -fsSL https://github.com/ricardoqsx/cktop/releases/download/dtop-v0.4.1/install-dtop.sh | sh -s -- --yes --scope user
```

The installer:

- detects Linux/macOS and AMD64/ARM64;
- asks whether to install for the current user or for the whole system;
- lets you confirm or change the binary directory;
- shows the final paths before continuing;
- verifies the embedded SHA-256 of the asset and, if Minisign is installed, the published signature;
- installs `dtop` and checks `dtop --version` before copying it;
- creates a complete configuration and its `.example`, without overwriting an existing configuration;
- asks for `sudo` only when writing a system-wide install.

Default paths:

| Scope | Binary | Configuration |
| --- | --- | --- |
| User | `~/.local/bin/dtop` | `${XDG_CONFIG_HOME:-~/.config}/dtop/dtop.conf` |
| System | `/usr/local/bin/dtop` | `/etc/dtop/dtop.conf` |

To install a different version, replace `0.4.1` in the URL with the published version. Each installer embeds the exact hashes of its own assets; do not use a `latest` URL.

**Updating:** run the installer of the new version. The binary is replaced and `dtop.conf` is preserved; the reference configuration is refreshed in `dtop.conf.example`.

**Uninstalling** a user install:

```bash
rm "$HOME/.local/bin/dtop"
rm -rf "${XDG_CONFIG_HOME:-$HOME/.config}/dtop"
```

A system uninstall uses the same paths with `sudo`; keep `/etc/dtop/dtop.conf` if you want to reuse the configuration.

**Verifying the download** — the manual assets, `SHA256SUMS`, `SHA256SUMS.minisig`, the public key `dtop.minisign.pub` and verifiable provenance are published on [GitHub Releases](https://github.com/ricardoqsx/cktop/releases):

```bash
minisign -Vm SHA256SUMS -x SHA256SUMS.minisig -p dtop.minisign.pub
asset=dtop_0.4.1_darwin_arm64.tar.gz
grep " $asset$" SHA256SUMS | shasum -a 256 -c -
```

Before trusting the signature, the downloaded public key must match `configs/dtop.minisign.pub` on the source tag of that version. On Linux you can replace `shasum -a 256` with `sha256sum`.

### Supported platforms

- Linux AMD64 and ARM64 with a local Docker Engine.
- WSL through the same Linux binary and an Engine reachable from the distribution.
- macOS Intel and Apple Silicon with Docker Desktop.

Native Windows, `.exe` binaries, PowerShell and Docker named-pipe connections are out of scope. Remote Docker over SSH or TLS is not advertised as compatible in this version.

### Features

`dtop` opens directly on the container list and refreshes automatically.

- **Views:** Containers, Stacks, Images, Networks and Volumes, switched with `left`/`right`.
- **Metrics:** CPU, memory, network, uptime and health per container, with a compact engine header.
- **Multi-select:** `e`, `space` and `a` let you act on several resources at once with a clear confirmation.
- **Updates:** detects new image versions for running containers; `U` means update available, `R` means a download is pending recreate, `=` is current, `P` is pinned by digest, `?` is unverifiable.
- **Docker Compose:** stacks are discovered from labels; optionally register local projects in `dtop.conf` so they remain visible even while `down`.
- **Advanced cleanup:** from any main view, `x` opens a global menu to prune stopped containers, images, networks, volumes or unused Docker data; it asks you to type `prune` before running.

Essential keys:

```text
left/right      switch views
up/down or j/k  select a resource
x               open the Advanced cleanup menu
o               cycle sorting
r               refresh
enter           open contextual actions, details, or expand a Stack
d               details for a container
l               follow logs
s               open an interactive shell
esc             stop logs, return, or collapse
e               multi-select mode
?               help
q               quit
```

**Configuration** is optional. To choose how memory is shown and enable update checks:

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

Full usage and every action are covered in the [user guide](userguide.md).

### Documentation

- [User guide](userguide.md): daily use, views, actions, updates and configuration.
- [Development notes](.proyects/documental_readme.md): state, local build, CLI and structure.

### License

Distributed under the [MIT license](LICENSE).

---

## Español

### ¿Qué es dtop?

`dtop` es una interfaz de terminal rápida y controlada por teclado que muestra qué está pasando en tu host Docker y te permite actuar sobre ello: contenedores, CPU y memoria, stacks activos, imágenes, redes, volúmenes y actualizaciones de imágenes disponibles, todo en una sola pantalla, sin daemon ni servicio web.

Funciona **sin configuración** y nunca reemplaza el CLI de Docker. Aporta contexto y ejecuta acciones concretas mediante tu Docker Engine local.

### Instalación

Instala la última versión estable con un solo comando:

```bash
curl -fsSL https://github.com/ricardoqsx/cktop/releases/download/dtop-v0.4.1/install-dtop.sh | sh
```

O con `wget`:

```bash
wget -qO- https://github.com/ricardoqsx/cktop/releases/download/dtop-v0.4.1/install-dtop.sh | sh
```

Para CI o una instalación no interactiva del usuario actual:

```bash
curl -fsSL https://github.com/ricardoqsx/cktop/releases/download/dtop-v0.4.1/install-dtop.sh | sh -s -- --yes --scope user
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

Rutas predeterminadas:

| Alcance | Binario | Configuración |
| --- | --- | --- |
| Usuario | `~/.local/bin/dtop` | `${XDG_CONFIG_HOME:-~/.config}/dtop/dtop.conf` |
| Sistema | `/usr/local/bin/dtop` | `/etc/dtop/dtop.conf` |

Para instalar otra versión, reemplaza `0.4.1` en la URL por la versión publicada. Cada instalador contiene los hashes exactos de sus propios assets; no uses una URL `latest`.

**Actualizar:** ejecuta el instalador de la nueva versión. El binario se reemplaza y `dtop.conf` se conserva; la configuración de referencia se actualiza en `dtop.conf.example`.

**Desinstalar** una instalación de usuario:

```bash
rm "$HOME/.local/bin/dtop"
rm -rf "${XDG_CONFIG_HOME:-$HOME/.config}/dtop"
```

La desinstalación de sistema usa las mismas rutas con `sudo`; conserva `/etc/dtop/dtop.conf` si quieres reutilizar la configuración.

**Verificar la descarga** — los assets manuales, `SHA256SUMS`, `SHA256SUMS.minisig`, la clave pública `dtop.minisign.pub` y la procedencia verificable se publican en [GitHub Releases](https://github.com/ricardoqsx/cktop/releases):

```bash
minisign -Vm SHA256SUMS -x SHA256SUMS.minisig -p dtop.minisign.pub
asset=dtop_0.4.1_darwin_arm64.tar.gz
grep " $asset$" SHA256SUMS | shasum -a 256 -c -
```

Antes de confiar en la firma, la clave pública descargada debe coincidir con `configs/dtop.minisign.pub` en el tag fuente de esa versión. En Linux puedes sustituir `shasum -a 256` por `sha256sum`.

### Plataformas soportadas

- Linux AMD64 y ARM64 con Docker Engine local.
- WSL mediante el mismo binario Linux y un Engine accesible desde la distribución.
- macOS Intel y Apple Silicon con Docker Desktop.

Windows nativo, ejecutables `.exe`, PowerShell y conexión mediante Docker named pipe quedan fuera del alcance. Docker remoto por SSH o TLS no se anuncia como compatible en esta versión.

### Características

`dtop` abre directamente sobre la lista de contenedores y se actualiza automáticamente.

- **Vistas:** Containers, Stacks, Images, Networks y Volumes, que se cambian con `left`/`right`.
- **Métricas:** CPU, memoria, red, uptime y salud por contenedor, con un encabezado compacto del Engine.
- **Selección múltiple:** `e`, `space` y `a` permiten actuar sobre varios recursos a la vez con confirmación clara.
- **Actualizaciones:** detecta versiones nuevas de las imágenes de contenedores en ejecución; `U` indica actualización disponible, `R` una descarga pendiente de recrear, `=` actual, `P` fijada por digest, `?` no verificable.
- **Docker Compose:** los stacks se descubren por labels; opcionalmente registra proyectos locales en `dtop.conf` para que sigan visibles incluso estando `down`.
- **Limpieza avanzada:** desde cualquier vista principal, `x` abre un menú global para limpiar contenedores detenidos, imágenes, redes, volúmenes o datos Docker sin uso; exige escribir `prune` antes de ejecutar.

Teclas esenciales:

```text
left/right      cambiar de vista
up/down o j/k   seleccionar un recurso
x               abrir el menú de limpieza avanzada
o               cambiar el orden
r               refrescar
enter           abrir acciones, detalles o expandir un Stack
d               detalles de un contenedor
l               seguir logs
s               abrir una shell interactiva
esc             detener logs, volver o colapsar
e               modo de selección múltiple
?               ayuda
q               salir
```

**Configuración** opcional. Para elegir cómo se muestra la memoria y activar las comprobaciones de actualizaciones:

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

El uso completo y todas las acciones están cubiertos en la [guía de usuario](guiausuario.md).

### Documentación

- [Guía de usuario](guiausuario.md): uso diario, vistas, acciones, actualizaciones y configuración.
- [Notas de desarrollo](.proyects/documental_readme.md): estado, build local, CLI y estructura.

### Licencia

Distribuido bajo la [licencia MIT](LICENSE).
