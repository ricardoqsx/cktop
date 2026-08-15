# dtop v0.4.0

> Superseded by `v0.4.1`: the application archives are valid, but two entries in this release's `SHA256SUMS` include an incorrect `dist/` prefix. Use `v0.4.1` for installation and complete manual verification.

First stable local-only release of `dtop`.

## Included

- Responsive Containers, Stacks, Images, Networks and Volumes views.
- Contextual lifecycle actions, logs, interactive shell and multi-selection.
- Safe Advanced cleanup operations with explicit `prune` confirmation.
- Image update detection and durable Docker Compose pull/apply coordination.
- English and Spanish interface.
- Layered configuration through system, XDG, `DTOP_CONFIG` and `--config`.
- Linux AMD64/ARM64, WSL through Linux, and macOS Intel/Apple Silicon builds.

## Install

The versioned installer asks whether to install for the current user or all users, confirms the binary destination, verifies the downloaded archive, installs `dtop`, and creates a complete configuration without overwriting an existing one.

```sh
curl -fsSL https://github.com/ricardoqsx/cktop/releases/download/dtop-v0.4.0/install-dtop.sh | sh
```

Docker remote endpoints and Windows native executables are outside this release scope.
