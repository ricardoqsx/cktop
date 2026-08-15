# dtop

`dtop` is a terminal interface for observing and operating a local Docker Engine and registered Docker Compose projects.

## Install This Archive

```sh
tar -xzf dtop_VERSION_OS_ARCH.tar.gz
install -m 0755 dtop "$HOME/.local/bin/dtop"
mkdir -p "${XDG_CONFIG_HOME:-$HOME/.config}/dtop"
cp dtop.conf.example "${XDG_CONFIG_HOME:-$HOME/.config}/dtop/dtop.conf"
```

Use `/usr/local/bin/dtop` and `/etc/dtop/dtop.conf` for a system-wide installation. Preserve an existing configuration when upgrading.

Run `dtop --help` for CLI options. Full usage and release documentation lives at <https://github.com/ricardoqsx/cktop>.

Supported platforms are Linux AMD64/ARM64, WSL through the Linux binary, and macOS Intel/Apple Silicon. Windows native is not supported.
