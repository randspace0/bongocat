#!/usr/bin/env bash
set -euo pipefail

BINARY=bongocat
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Defaults: user-level install (no root required).
PREFIX="${HOME}/.local"
SYSTEM=false

usage() {
    echo "Usage: $0 [--system] [--prefix PREFIX] [--uninstall]"
    echo ""
    echo "  --system          Install system-wide to /usr/local (requires sudo)"
    echo "  --prefix PREFIX   Override install prefix (default: ~/.local)"
    echo "  --uninstall       Remove a previously installed BongoCat"
    exit 1
}

UNINSTALL=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --system)   SYSTEM=true; PREFIX=/usr/local; shift ;;
        --prefix)   PREFIX="$2"; shift 2 ;;
        --uninstall) UNINSTALL=true; shift ;;
        -h|--help)  usage ;;
        *) echo "Unknown option: $1"; usage ;;
    esac
done

BIN_DIR="${PREFIX}/bin"
ICON_DIR="${PREFIX}/share/icons/hicolor/256x256/apps"
DESKTOP_DIR="${PREFIX}/share/applications"

uninstall() {
    echo "Uninstalling BongoCat..."
    rm -f "${BIN_DIR}/${BINARY}"
    rm -f "${ICON_DIR}/${BINARY}.png"
    rm -f "${DESKTOP_DIR}/${BINARY}.desktop"
    update-desktop-database "${DESKTOP_DIR}" 2>/dev/null || true
    gtk-update-icon-cache -f -t "${PREFIX}/share/icons/hicolor" 2>/dev/null || true
    echo "Uninstall complete."
    exit 0
}

[[ "${UNINSTALL}" == true ]] && uninstall

# Check dependencies
if ! command -v go &>/dev/null; then
    echo "error: 'go' not found. Install Go 1.21+ from https://go.dev/dl/" >&2
    exit 1
fi

if ! pkg-config --exists ayatana-appindicator3-0.1 2>/dev/null; then
    echo "error: missing libayatana-appindicator3-dev. Install it with:"
    echo "  sudo apt-get install libayatana-appindicator3-dev"
    exit 1
fi

# Build
echo "Building..."
cd "${REPO_DIR}"
go build -ldflags="-s -w" -o "${BINARY}" .
echo "Build complete."

# Install binary
mkdir -p "${BIN_DIR}"
cp "${BINARY}" "${BIN_DIR}/${BINARY}"
chmod 755 "${BIN_DIR}/${BINARY}"
echo "Installed binary  → ${BIN_DIR}/${BINARY}"

# Install icon (256×256 slot; the image is 397×201 but DEs handle non-square fine)
mkdir -p "${ICON_DIR}"
cp "${REPO_DIR}/assets/base.png" "${ICON_DIR}/${BINARY}.png"
echo "Installed icon    → ${ICON_DIR}/${BINARY}.png"

# Install desktop entry with the real binary path substituted
mkdir -p "${DESKTOP_DIR}"
sed "s|^Exec=bongocat|Exec=${BIN_DIR}/${BINARY}|" \
    "${REPO_DIR}/bongocat.desktop" > "${DESKTOP_DIR}/${BINARY}.desktop"
echo "Installed desktop → ${DESKTOP_DIR}/${BINARY}.desktop"

# Refresh caches
update-desktop-database "${DESKTOP_DIR}" 2>/dev/null || true
gtk-update-icon-cache -f -t "${PREFIX}/share/icons/hicolor" 2>/dev/null || true

# Warn if bin dir is not in PATH
if [[ ":${PATH}:" != *":${BIN_DIR}:"* ]]; then
    echo ""
    echo "Note: ${BIN_DIR} is not in your PATH."
    echo "Add this to your shell profile:"
    echo "  export PATH=\"\${HOME}/.local/bin:\${PATH}\""
fi

echo ""
echo "Installation complete. Launch BongoCat from your applications menu or run:"
echo "  ${BIN_DIR}/${BINARY}"
