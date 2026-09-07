# SSH Tunnel Manager (`sshtm`)

SSH Tunnel Manager provides a CLI and background daemon for saving, starting, listing, and stopping SSH local port-forwarding tunnels.

## Requirements

- Go 1.25 or later for source builds
- Git
- A populated `~/.ssh/known_hosts` entry for each SSH server
- Protocol Buffers compiler 35.0 for protobuf development

## Installation

```sh
git clone https://github.com/besrabasant/ssh-tunnel-manager.git
cd ssh-tunnel-manager
./install.sh
```

The install script installs `sshtm` and `sshtmd` under `~/.local/bin`, configuration under `~/.ssh-tunnel-manager`, and helper scripts under `~/.local/share/sshtm/scripts`. It also installs a systemd user service on Linux or a LaunchAgent on macOS.

Ensure `~/.local/bin` is on your `PATH`.

### Arch Linux (AUR)

```sh
yay -S sshtm
```

### Debian/Ubuntu package build

```sh
cd packaging/debian
dpkg-buildpackage -us -uc
sudo dpkg -i ../sshtm_1.1.5-1_amd64.deb
```

## Uninstallation

```sh
~/.local/share/sshtm/scripts/uninstall.sh
```

## Quick start

```sh
sshtm machine add my_server --server example.com --user alice --key-file ~/.ssh/id_ed25519
sshtm add
sshtm list
sshtm tunnel my_configuration
sshtm active
sshtm kill my_configuration
```

A machine stores reusable SSH connection details. A tunnel profile stores a port-forwarding configuration and references a machine. Create a machine first; `sshtm add` creates a tunnel profile and prompts you to select an existing machine.

## Commands

```sh
sshtm [command]
```

| Command | Aliases | Description |
| --- | --- | --- |
| `list [search pattern]` | `ls`, `l` | List saved tunnel profiles. |
| `add` | `a` | Add a tunnel profile interactively. |
| `edit` | `e` | Edit a tunnel profile. |
| `delete` | `del`, `d` | Delete a tunnel profile. |
| `machine list` | | List SSH machines. |
| `machine add <name> --server <address> --user <user> --key-file <path>` | | Add an SSH machine. |
| `machine edit <name>` | | Edit an SSH machine. |
| `machine delete <name>` | | Delete an SSH machine. |
| `tunnel <configuration name> [local port]` | `t` | Start a saved tunnel. |
| `active` | | List active tunnels. |
| `kill <configuration name\|local port>` | `k`, `terminate` | Terminate an active tunnel. |
| `completion` | | Generate shell completions. |
| `version` | | Print the version. |

## Configuration directory

Set `SSHTM_CONFIG_DIR` to use a different configuration directory:

```sh
SSHTM_CONFIG_DIR=/path/to/config sshtm list
```

The legacy `config-dir` environment variable remains supported for compatibility. `SSHTM_CONFIG_DIR` takes precedence when both are set.

Old flat JSON configurations are migrated automatically and retained. New machine and tunnel-profile data is stored in `machines/` and `profiles/` under the configuration directory.

## Development

Install the pinned protobuf Go generators once:

```sh
make proto-tools
```

After editing `rpc/daemon.proto`, regenerate generated code. Format and run the full local check before submitting changes:

```sh
make proto
make fmt
make check
```

`make check` runs formatting checks, `go vet`, normal and race-detector tests, builds, and protobuf freshness checks. `make coverage` writes `coverage.out` and reports coverage for each tested package.

Build local binaries:

```sh
make build
```

This writes `sshtm` and `sshtmd` to `./bin`. Build all supported release binaries with:

```sh
make VERSION=1.1.5 build-all
```

This writes Linux and macOS binaries to `./dist`. Override the output directories when needed:

```sh
BIN_DIR=/tmp/sshtm-bin make build
DIST_DIR=/tmp/sshtm-dist make build-all
```

Use `air` for optional daemon live reload, or run the CLI directly:

```sh
cd client
go run main.go list
```

## Project layout

```text
client/      CLI commands and terminal UI
daemon/      Background daemon and gRPC server
pkg/         Reusable configuration and tunnel-management packages
rpc/         Protobuf definitions and generated gRPC code
config/      Application constants and version metadata
packaging/   Arch Linux and Debian packaging files
scripts/     Install and uninstall helpers
utils/       Shared utility helpers
```

## License

This project is licensed under the [MIT License](LICENSE).
