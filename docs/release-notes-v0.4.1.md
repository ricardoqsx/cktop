# dtop v0.4.1

Recommended first stable local-only release of `dtop`.

This patch supersedes `v0.4.0` by correcting the `SHA256SUMS` names for `install-dtop.sh` and `dtop.minisign.pub`. Application behavior and supported platforms are unchanged.

## Install

The versioned installer asks for user or system scope, confirms the destination, verifies the downloaded archive, installs `dtop`, and creates a complete configuration without overwriting an existing one.

```sh
curl -fsSL https://github.com/ricardoqsx/cktop/releases/download/dtop-v0.4.1/install-dtop.sh | sh
```

Supported platforms are Linux AMD64/ARM64, WSL through Linux, and macOS Intel/Apple Silicon. Docker remote endpoints and Windows native executables remain outside this release scope.
