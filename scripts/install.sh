#!/bin/sh
# ody installer.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/exmorphe/odyssey-beta/master/scripts/install.sh | sh
#
# Detects OS (darwin/linux) and CPU (amd64/arm64), downloads the matching
# release tarball from github.com/exmorphe/odyssey-beta, verifies its
# SHA256 against checksums.txt, and installs ody to ~/.local/bin.
#
# Environment overrides:
#   ODY_VERSION       Release tag to install (default: latest)
#   ODY_INSTALL_DIR   Where to drop the binary (default: $HOME/.local/bin)
#   ODY_BASE_URL      Override release base URL (test/dev only)
#   ODY_OS, ODY_ARCH  Force OS/arch detection (test only)

set -eu

REPO="exmorphe/odyssey-beta"
DEFAULT_BASE="https://github.com/${REPO}/releases"

err() { printf 'install: %s\n' "$*" >&2; }
die() { err "$*"; exit 1; }

need() {
    command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"
}

detect_os() {
    if [ -n "${ODY_OS:-}" ]; then
        printf '%s' "$ODY_OS"
        return
    fi
    case "$(uname -s)" in
        Darwin) printf 'darwin' ;;
        Linux)  printf 'linux'  ;;
        *) die "unsupported OS: $(uname -s). Supported: darwin, linux." ;;
    esac
}

detect_arch() {
    if [ -n "${ODY_ARCH:-}" ]; then
        printf '%s' "$ODY_ARCH"
        return
    fi
    case "$(uname -m)" in
        x86_64|amd64)  printf 'amd64' ;;
        arm64|aarch64) printf 'arm64' ;;
        *) die "unsupported arch: $(uname -m). Supported: amd64, arm64." ;;
    esac
}

sha256_file() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | awk '{print $1}'
    else
        die "no sha256 tool available (need sha256sum or shasum)"
    fi
}

main() {
    need curl
    need tar
    need uname
    need mktemp
    need awk

    OS="$(detect_os)"
    ARCH="$(detect_arch)"
    case "$OS" in
        darwin|linux) ;;
        *) die "unsupported OS: $OS. Supported: darwin, linux." ;;
    esac
    case "$ARCH" in
        amd64|arm64) ;;
        *) die "unsupported arch: $ARCH. Supported: amd64, arm64." ;;
    esac

    VERSION="${ODY_VERSION:-latest}"
    INSTALL_DIR="${ODY_INSTALL_DIR:-$HOME/.local/bin}"
    BASE="${ODY_BASE_URL:-$DEFAULT_BASE}"

    if [ "$VERSION" = "latest" ]; then
        BIN_URL="$BASE/latest/download/ody_${OS}_${ARCH}.tar.gz"
        SUM_URL="$BASE/latest/download/checksums.txt"
    else
        BIN_URL="$BASE/download/$VERSION/ody_${OS}_${ARCH}.tar.gz"
        SUM_URL="$BASE/download/$VERSION/checksums.txt"
    fi

    TMP="$(mktemp -d)"
    trap 'rm -rf "$TMP"' EXIT INT TERM

    err "downloading $BIN_URL"
    curl -fsSL --retry 3 -o "$TMP/ody.tar.gz" "$BIN_URL" \
        || die "download failed: $BIN_URL"
    curl -fsSL --retry 3 -o "$TMP/checksums.txt" "$SUM_URL" \
        || die "download failed: $SUM_URL"

    ASSET="ody_${OS}_${ARCH}.tar.gz"
    EXPECTED="$(awk -v f="$ASSET" '
        {
            name = $2
            sub(/^\*/, "", name)
            if (name == f) { print $1; exit }
        }
    ' "$TMP/checksums.txt")"
    [ -n "$EXPECTED" ] || die "no checksum entry for $ASSET in checksums.txt"

    ACTUAL="$(sha256_file "$TMP/ody.tar.gz")"
    [ "$EXPECTED" = "$ACTUAL" ] \
        || die "checksum mismatch for $ASSET (expected $EXPECTED, got $ACTUAL)"

    mkdir -p "$INSTALL_DIR" || die "cannot create $INSTALL_DIR"
    tar -xzf "$TMP/ody.tar.gz" -C "$INSTALL_DIR" ody \
        || die "tar extract failed"
    chmod +x "$INSTALL_DIR/ody"

    printf '\nInstalled ody to %s/ody\n' "$INSTALL_DIR"
    case ":$PATH:" in
        *":$INSTALL_DIR:"*) ;;
        *) printf 'Note: %s is not on your PATH. Add it to your shell profile.\n' "$INSTALL_DIR" ;;
    esac
    printf '\nNext: ody login https://k8sodyssey.com\n'
}

main "$@"
