#!/bin/bash
set -euo pipefail

log_step() {
	echo "🚀 [sshtm] $1"
}

log_step "Starting installation"

REQUIRED_GO_VERSION="1.26"

command -v go >/dev/null 2>&1 || {
	echo "Go ${REQUIRED_GO_VERSION} or newer is required to build sshtm" >&2
	exit 1
}

GO_VERSION="$(go env GOVERSION | sed -E 's/^go([0-9]+\.[0-9]+).*/\1/')"
if ! awk -v installed="$GO_VERSION" -v required="$REQUIRED_GO_VERSION" 'BEGIN {
	split(installed, i, "."); split(required, r, ".");
	exit (i[1] > r[1] || (i[1] == r[1] && i[2] >= r[2])) ? 0 : 1
}'; then
	echo "Go ${REQUIRED_GO_VERSION} or newer is required (found ${GO_VERSION})" >&2
	exit 1
fi

command -v ssh >/dev/null 2>&1 || {
	echo "OpenSSH (ssh) is required to run sshtm" >&2
	exit 1
}

# Paths (all user-scoped)
DATA_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/sshtm" # application configuration root

# Detect OS/arch for service management and binary selection
OS_NAME="$(uname -s)"
ARCH_NAME="$(uname -m)"
case "$ARCH_NAME" in
	x86_64|amd64)
		ARCH_NAME="amd64"
		;;
	arm64|aarch64)
		ARCH_NAME="arm64"
		;;
	*)
		echo "Unsupported architecture: $ARCH_NAME" >&2
		exit 1
		;;
esac
case "$OS_NAME" in
	Darwin|Linux) ;;
	*)
		echo "Unsupported operating system: $OS_NAME" >&2
		exit 1
		;;
esac
if [ "$OS_NAME" = "Darwin" ]; then
	LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"   # launchd user agents directory
	SHARE_DIR="$HOME/Library/Application Support/sshtm" # application shared assets root
	BIN_DIR="$HOME/.local/bin"                       # user binaries directory
	DAEMON_BIN="$BIN_DIR/sshtmd"                     # sshtm daemon binary
	CLIENT_BIN="$BIN_DIR/sshtm"                      # sshtm client binary
	SCRIPTS_DIR="$SHARE_DIR/scripts"                 # bundled scripts directory
	PLIST_FILE="$LAUNCH_AGENTS_DIR/com.sshtm.daemon.plist" # launchd plist path
else
	SYSTEMD_USER_DIR="$HOME/.config/systemd/user"  # systemd user unit directory
	SHARE_DIR="$HOME/.local/share/sshtm"           # application shared assets root
	BIN_DIR="$HOME/.local/bin"                     # user binaries directory
	DAEMON_BIN="$BIN_DIR/sshtmd"                   # sshtm daemon binary
	CLIENT_BIN="$BIN_DIR/sshtm"                    # sshtm client binary
	SCRIPTS_DIR="$SHARE_DIR/scripts"               # bundled scripts directory
	UNIT_FILE="$SYSTEMD_USER_DIR/sshtmd.service"   # systemd unit file path
fi

log_step "🔍 Detected $OS_NAME/$ARCH_NAME"

# Build binaries for the detected OS/arch. Generated protobuf sources are
# committed, so end users do not need protoc or its Go plugins.
TARGET_OS=""
if [ "$OS_NAME" = "Darwin" ]; then
	TARGET_OS="darwin"
else
	TARGET_OS="linux"
fi
SRC_DAEMON="sshtmd-${TARGET_OS}-${ARCH_NAME}"
SRC_CLIENT="sshtm-${TARGET_OS}-${ARCH_NAME}"
log_step "🔨 Building daemon binary: $SRC_DAEMON"
GOOS="$TARGET_OS" GOARCH="$ARCH_NAME" go build -o "$SRC_DAEMON" ./daemon
log_step "🔨 Building client binary: $SRC_CLIENT"
GOOS="$TARGET_OS" GOARCH="$ARCH_NAME" go build -o "$SRC_CLIENT" ./client

# Create required directories
log_step "📁 Creating application directories"
if [ "$OS_NAME" = "Darwin" ]; then
	mkdir -p "$LAUNCH_AGENTS_DIR"
else
	mkdir -p "$SYSTEMD_USER_DIR"
fi
mkdir -p "$DATA_DIR"
mkdir -p "$SHARE_DIR"
mkdir -p "$BIN_DIR"

# Stop existing service before replacing binaries
log_step "⏹️ Stopping existing daemon service, if running"
if [ "$OS_NAME" = "Darwin" ]; then
	if launchctl list | grep -q "com.sshtm.daemon"; then
		launchctl unload "$PLIST_FILE" || true
	fi
else
	if systemctl --user is-active --quiet sshtmd; then
		systemctl --user stop sshtmd
	fi
fi

# Install/overwrite binaries in-place (updates existing installations)
log_step "📥 Installing binaries into $BIN_DIR"
rm -f "$DAEMON_BIN" "$CLIENT_BIN"
install -m 755 "$SRC_DAEMON" "$DAEMON_BIN"
install -m 755 "$SRC_CLIENT" "$CLIENT_BIN"

# Refresh bundled scripts directory
log_step "📜 Installing bundled scripts into $SCRIPTS_DIR"
rm -rf "$SCRIPTS_DIR"
cp -r scripts "$SHARE_DIR/"

# Register and start the daemon as a user service
if [ "$OS_NAME" = "Darwin" ]; then
	log_step "🍎 Registering and starting launchd agent"
	cat <<EOF > "$PLIST_FILE"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.sshtm.daemon</string>
	<key>ProgramArguments</key>
	<array>
		<string>$DAEMON_BIN</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
</dict>
</plist>
EOF

	# Load/enable launchd agent
	launchctl load "$PLIST_FILE"
else
	log_step "⚙️ Registering systemd user service"
	cat <<EOF > "$UNIT_FILE"
[Unit]
Description=SSH Tunnel Manager Daemon

[Service]
Type=simple
ExecStart=$DAEMON_BIN

[Install]
WantedBy=default.target
EOF

	# Reload, enable, and start systemd user service
	log_step "🔄 Reloading systemd user units"
	systemctl --user daemon-reload

	# Enable and start the service
	log_step "✅ Enabling sshtmd service"
	systemctl --user enable sshtmd

	# If the service is already running, restart it. Otherwise start it normally.
	if systemctl --user is-active --quiet sshtmd; then
		log_step "🔁 Restarting sshtmd service"
		systemctl --user restart sshtmd
	else
		log_step "▶️ Starting sshtmd service"
		systemctl --user start sshtmd
	fi
fi

log_step "🎉 Installation complete"
