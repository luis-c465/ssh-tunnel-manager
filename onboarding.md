# Onboarding Guide

SSH Tunnel Manager (`sshtm`) is a CLI and background daemon for managing saved SSH tunnel configurations.

## Repository map

```text
client/      CLI commands and terminal UI
daemon/      gRPC daemon and task handlers
pkg/         Configuration and tunnel-management packages
rpc/         Protobuf schema and generated gRPC code
config/      Application constants and configuration directory selection
scripts/     Install and uninstall helpers
```

Start with `rpc/daemon.proto` for the client-daemon API, `daemon/tasks/` for daemon operations, and `client/cmd/` for CLI commands.

## Development setup

Requires Go 1.26+, Git, and Protocol Buffers compiler 36.1. Install the pinned Go protobuf generators once:

```sh
make proto-tools
```

After editing `rpc/daemon.proto`, use this development loop:

```sh
make proto
make fmt
make check
```

`make check` runs formatting checks, `go vet`, normal and race-detector tests, builds, and protobuf freshness checks. To create `coverage.out` and report coverage for each tested package, run:

```sh
make coverage
```

## Build outputs

```sh
make build
```

Local `sshtm` and `sshtmd` binaries are written to `./bin`.

```sh
make VERSION=1.1.5 build-all
```

Cross-platform Linux and macOS binaries are written to `./dist`. Override those directories when needed:

```sh
BIN_DIR=/tmp/sshtm-bin make build
DIST_DIR=/tmp/sshtm-dist make build-all
```

Run `make clean` to remove `./bin`, `./dist`, and `coverage.out`.

## Configuration directory

The default directory is `${XDG_CONFIG_HOME:-$HOME/.config}/sshtm`. On daemon startup, legacy `~/.ssh-tunnel-manager` data is copied to that directory when the target is empty; if both locations contain data, the XDG directory is used without merging them.

Set `SSHTM_CONFIG_DIR` to choose the directory used by both the CLI and daemon:

```sh
SSHTM_CONFIG_DIR=/path/to/config sshtm list
```

The legacy `config-dir` environment variable remains supported for compatibility. `SSHTM_CONFIG_DIR` takes precedence. Neither explicit override triggers legacy-directory relocation.

`./install.sh` installs the user service and production binaries; it is separate from the local development build commands above.
