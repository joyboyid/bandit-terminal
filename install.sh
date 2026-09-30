#!/bin/sh
# Install bandit for the current user, or system-wide with --system.
# User install lands in ~/.local, which dmenu, rofi, and other launchers
# search without root. System install uses /usr/local.
set -eu

cd "$(dirname "$0")"

mode=user
if [ "${1:-}" = "--system" ]; then
	mode=system
elif [ "${1:-}" = "--uninstall" ]; then
	mode=uninstall-user
elif [ "${1:-}" = "--uninstall-system" ]; then
	mode=uninstall-system
elif [ -n "${1:-}" ]; then
	echo "usage: ./install.sh [--system | --uninstall | --uninstall-system]" >&2
	exit 1
fi

case "$mode" in
user) prefix="${PREFIX:-$HOME/.local}" ;;
system) prefix="${PREFIX:-/usr/local}" ;;
uninstall-user) prefix="${PREFIX:-$HOME/.local}" ;;
uninstall-system) prefix="${PREFIX:-/usr/local}" ;;
esac

remove() {
	rm -f "$prefix/bin/bandit"
	rm -f "$prefix/share/applications/bandit-terminal.desktop"
	rm -f "$prefix/share/icons/hicolor/"*"/apps/bandit-terminal.png"
	rm -f "$HOME/.cache/dmenu_run"
	if command -v update-desktop-database >/dev/null 2>&1; then
		update-desktop-database "$prefix/share/applications" >/dev/null 2>&1 || true
	fi
}

if [ "$mode" = uninstall-user ] || [ "$mode" = uninstall-system ]; then
	remove
	echo "removed bandit from $prefix"
	exit 0
fi

if ! command -v go >/dev/null 2>&1; then
	echo "go is required to build bandit" >&2
	exit 1
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
go build -o "$tmp" .

install -Dm755 "$tmp" "$prefix/bin/bandit"

icon="$prefix/share/icons/hicolor/256x256/apps/bandit-terminal.png"
found=0
for dir in share/icons/hicolor/*; do
	size="$(basename "$dir")"
	src="$dir/apps/bandit-terminal.png"
	[ -f "$src" ] || continue
	install -Dm644 "$src" "$prefix/share/icons/hicolor/$size/apps/bandit-terminal.png"
	found=1
done
if [ "$found" -ne 1 ]; then
	echo "icon pngs are missing under share/icons/hicolor" >&2
	exit 1
fi

desktop="$prefix/share/applications/bandit-terminal.desktop"
install -Dm644 share/applications/bandit-terminal.desktop "$desktop"
# An absolute icon path works even when the icon theme has no index.
sed -i "s|^Icon=.*|Icon=$icon|" "$desktop"

if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database "$prefix/share/applications" >/dev/null 2>&1 || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1 && [ -f "$prefix/share/icons/hicolor/index.theme" ]; then
	gtk-update-icon-cache -f "$prefix/share/icons/hicolor" >/dev/null 2>&1 || true
fi
rm -f "$HOME/.cache/dmenu_run"

echo "installed $prefix/bin/bandit"
echo "launcher $desktop"
