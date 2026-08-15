# dtop User Guide

This guide explains what `dtop` is for, how to move around the application, and what to expect from each action. You do not need to know the code to use it.

## 1. What is dtop

`dtop` is a TUI for observing and operating a Docker Engine from the terminal. Its goal is to answer quickly:

- which containers are running;
- which ones consume more CPU or memory;
- which Docker resources exist and which are in use;
- which Docker Compose projects are active;
- which images have an update available.

`dtop` does not replace the Docker CLI nor edit Compose files. It presents context and performs concrete actions using the Docker Engine or Docker CLI.

## 2. Starting and quitting

After [installing dtop](README.md), run it from any directory:

```bash
dtop
```

Press `q` to quit.

Current requirements:

- an accessible local Docker Engine;
- the Docker CLI available on `PATH`;
- Buildx recommended to compare multi-architecture images correctly.

## 3. Basic navigation

| Key | Action |
| --- | --- |
| `left` / `right` | Change view |
| `up` / `down` or `j` / `k` | Move the selection |
| `enter` | Open actions or expand a Stack depending on the view |
| `d` | Open details for the focused resource when available |
| `e` | Enter or leave multi-selection mode |
| `space` | Check or uncheck in multi-selection mode |
| `a` | In edit mode, check or uncheck all checkboxes |
| `r` | Refresh the active view |
| `x` | Open the global Advanced menu from a main view |
| `?` | Open or close Help |
| `Esc` | Go back, cancel, or close the current panel |
| `q` | Quit |

Focused rows are highlighted across the full width. In multi-selection mode a checkbox appears on each row.

## 4. Views

### Containers

Shows the state, health, CPU, memory, uptime and image of each container.

- `o` cycles the sort order among State, CPU, Memory and Name.
- `enter` opens Stop, Restart, Delete and Cancel for the focused container.
- `d` opens details.
- `l` follows the latest logs.
- `s` opens `/bin/sh -l` inside a running container.
- `e`, `space` and `enter` allow Stop, Restart or Delete on one or more containers.

A `U` mark next to the name indicates an update is available; `R` indicates a download pending to apply. The context menu adds `Pull update`, `Update now` or `Apply downloaded update` depending on the state. In multi-selection only eligible containers are processed and the menu reports the skipped ones.

The shell uses the real terminal. Return to dtop with `Ctrl+D` or `exit`; while it is open, `Esc` belongs to the shell.

### Stacks

Groups containers by Docker Compose project.

- `enter` expands or collapses the stack.
- The bottom panel shows the directory, the Compose files and action availability.
- `l` on the parent opens Compose logs.
- `l` or `s` on a child uses the focused container.
- The Up, Stop, Restart and Down actions only appear when usable local Compose metadata exists.

Down never adds `--volumes`.

### Images

Shows the name, update state, size, usage, age and ID when the width allows it.

`enter` opens the image actions. `e`, `space` and `enter` apply actions in batch.

### Networks and Volumes

Show identity, driver, usage and resource details.

- `enter` opens Delete or Cancel for the focused resource.
- `d` opens details.
- `e`, `space` and `enter` allow batch Delete.
- Docker rejects connected networks and referenced volumes.
- Deleting a volume can erase persistent data.

## 5. Confirmations

Actions that change containers or resources show a blue bar at the bottom of the application with the action, target and Engine.

```text
CONFIRM: Recreate containers
Target: example:latest on LOCAL desktop-linux
Are you sure? [y/N] | [Esc] cancel
```

- `y` confirms;
- `n` or `Esc` cancels;
- the confirmation stays visible until you decide.

`Pull update` does not ask for confirmation because it only downloads an image. It does not restart or replace containers.

## 6. Advanced cleanup

Press `x` from any of the five main views to open Advanced. It does not open over Help, details, logs, multi-selection or action menus.

The menu offers these local operations and shows the exact command of the focused option:

| Option | Command |
| --- | --- |
| Delete stopped containers | `docker container prune --force` |
| Delete unused images | `docker image prune --all --force` |
| Delete unused networks | `docker network prune --force` |
| Delete unused volumes | `docker volume prune --force` |
| Delete unused Docker data | `docker system prune --all --force` |

Before running you must type exactly `prune` and press `Enter`. `Esc` cancels. The system prune does not include `--volumes`; volumes are cleaned only with the separate option. When finished, the result stays visible and dtop reloads Containers, Images, Networks and Volumes. Remote endpoints remain rejected.

## 7. Image updates

The UPDATE column uses these indicators:

| State | Meaning |
| --- | --- |
| `U` | A different remote digest exists; an update is available |
| `R` | The new image is already downloaded and the container needs a recreate |
| `=` | The local and remote references match |
| `P` | The reference is pinned by digest |
| `?` | It was not possible to compare safely |
| `...` | The query is in progress |

The normal flow for a direct container is:

```text
U -> Pull update -> R -> Recreate containers -> =
```

1. On a `U` row, press `enter` and choose `Pull update`.
2. Pull starts immediately.
3. The row becomes `R` when the container is still using the previous image ID.
4. Press `enter`, choose `Recreate containers` and confirm with `y`.
5. dtop rebuilds the container with the downloaded image and rechecks the digest.

Direct recreation preserves the inspected container configuration. If the target belongs to Compose, dtop deduplicates children by Stack and service and validates the image against the effective Compose configuration. Before the pull it saves a safe marker and afterwards it persists the downloaded digest in `$XDG_STATE_HOME/dtop/compose-updates.json`, with the fallback `~/.local/state/dtop/compose-updates.json`.

A Compose Pull can survive a dtop restart and a `docker compose down`. While the downloaded digest differs from the applied one, a Down or Stopped Stack shows `Apply downloaded update` instead of Up. Apply requires `y`, revalidates configuration and local image, runs `up -d` and only marks the digest as applied after inspecting the resulting containers. If the state cannot be read or verified, Up fails safely. Build-managed, pinned, one-off or non-deterministic pull-policy services are not presented as updatable.

## 8. Docker Hub and the `?` state

Docker Hub limits anonymous queries. When the quota runs out, dtop shows `?` instead of claiming a wrong state.

Log in once:

```bash
docker login
```

dtop reuses the configuration and credential helpers of the Docker CLI. It does not store or show tokens.

Help shows this reminder while it does not detect a configured Docker Hub credential:

```text
Image updates
To get updates for your container images, log in to Docker Hub with: docker login
```

The check runs again each time Help is opened, so the message can disappear without restarting dtop after logging in.

The same notice appears on the main screen, just above the bottom help line, until a usable session is detected.

## 9. Optional configuration

Paths, in order of precedence:

1. `/etc/dtop/dtop.conf`;
2. `$XDG_CONFIG_HOME/dtop/dtop.conf`;
3. `~/.config/dtop/dtop.conf`.

Example:

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

`memory_mode` accepts `usage`, `percent` or `both`. Colors accept ANSI/256 values from `0` to `255`.

## 10. Errors and partial data

- Containers refresh every two seconds.
- Images, Networks and Volumes reconcile every five seconds.
- If a source fails, dtop keeps its last rows and marks them as known but not current data.
- A registry failure produces `?`, not a false `=`.
- Action results show how many targets finished and how many failed.

## 11. Current limitations

- A single local Docker Engine per run.
- Compose updates require valid local Stack metadata.
- Version history and rollback are not implemented yet.
- Bind mounts of a Compose are not persisted yet after Down.
- Docker Hub may limit queries without login.
