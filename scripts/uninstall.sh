#!/usr/bin/env bash
set -euo pipefail

case "$(uname -s)" in
	Darwin)
		plist="$HOME/Library/LaunchAgents/com.sshtm.daemon.plist"
		if [ -f "$plist" ]; then
			launchctl unload "$plist" 2>/dev/null || true
			rm -f "$plist"
		fi
		rm -rf "$HOME/Library/Application Support/sshtm"
		;;
	Linux)
		if command -v systemctl >/dev/null 2>&1; then
			systemctl --user stop sshtmd 2>/dev/null || true
			systemctl --user disable sshtmd 2>/dev/null || true
		fi
		rm -f "$HOME/.config/systemd/user/sshtmd.service"
		rm -rf "$HOME/.local/share/sshtm"
		if command -v systemctl >/dev/null 2>&1; then
			systemctl --user daemon-reload 2>/dev/null || true
		fi
		;;
	*)
		echo "Unsupported operating system" >&2
		exit 1
		;;
esac

rm -f "$HOME/.local/bin/sshtmd" "$HOME/.local/bin/sshtm"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/sshtm"
printf 'Uninstalled sshtm. Configuration data in %s was preserved.\n' "$CONFIG_DIR"
