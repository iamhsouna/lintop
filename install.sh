#!/usr/bin/env bash
#
# lintop installer / updater
#
# One-line install:
#   curl -fsSL https://raw.githubusercontent.com/iamhsouna/lintop/main/install.sh | bash
#
# One-line update:
#   curl -fsSL https://raw.githubusercontent.com/iamhsouna/lintop/main/install.sh | bash -s -- --update
#
# Options:
#   --update             Reinstall/upgrade to the latest release
#   --version <tag>      Install a specific release tag (e.g. v2.2.0)
#   --prefix <dir>       Install directory (default: /usr/local/bin or ~/.local/bin)
#   --from-source        Build from source instead of downloading a release
#   --uninstall          Remove the installed lintop binary
#   -h, --help           Show this help
#
set -euo pipefail

REPO="iamhsouna/lintop"
BIN="lintop"
VERSION="latest"
PREFIX=""
FROM_SOURCE=0
UNINSTALL=0

C_RESET='\033[0m'; C_BOLD='\033[1m'; C_GREEN='\033[32m'
C_YELLOW='\033[33m'; C_RED='\033[31m'; C_CYAN='\033[36m'

info()  { printf "${C_CYAN}==>${C_RESET} %s\n" "$*"; }
ok()    { printf "${C_GREEN}✓${C_RESET} %s\n" "$*"; }
warn()  { printf "${C_YELLOW}!${C_RESET} %s\n" "$*"; }
die()   { printf "${C_RED}x${C_RESET} %s\n" "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
lintop installer / updater

One-line install:
  curl -fsSL https://raw.githubusercontent.com/iamhsouna/lintop/main/install.sh | bash

One-line update:
  curl -fsSL https://raw.githubusercontent.com/iamhsouna/lintop/main/install.sh | bash -s -- --update

Options:
  --update             Reinstall/upgrade to the latest release
  --version <tag>      Install a specific release tag (e.g. v2.2.0)
  --prefix <dir>       Install directory (default: /usr/local/bin or ~/.local/bin)
  --from-source        Build from source instead of downloading a release
  --uninstall          Remove the installed lintop binary
  -h, --help           Show this help
EOF
  exit 0
}

while [ $# -gt 0 ]; do
  case "$1" in
    --update)       VERSION="latest" ;;
    --version)      shift; [ $# -ge 1 ] || die "--version needs a tag"; VERSION="$1" ;;
    --version=*)    VERSION="${1#*=}" ;;
    --prefix)       shift; [ $# -ge 1 ] || die "--prefix needs a directory"; PREFIX="$1" ;;
    --prefix=*)     PREFIX="${1#*=}" ;;
    --from-source)  FROM_SOURCE=1 ;;
    --uninstall)    UNINSTALL=1 ;;
    -h|--help)      usage ;;
    *)              die "unknown option: $1 (try --help)" ;;
  esac
  shift
done

need() { command -v "$1" >/dev/null 2>&1; }

# ---------------------------------------------------------------------------
# Platform detection
# ---------------------------------------------------------------------------
os="$(uname -s)"
case "$os" in
  Linux) ;;
  Darwin) die "lintop targets Linux. Use mactop on macOS: https://github.com/metaspartan/mactop" ;;
  *) die "unsupported operating system: $os" ;;
esac

case "$(uname -m)" in
  x86_64|amd64)        ARCH="amd64"; ARCH_ALT="x86_64" ;;
  aarch64|arm64)       ARCH="arm64"; ARCH_ALT="aarch64" ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac

# ---------------------------------------------------------------------------
# Install directory
# ---------------------------------------------------------------------------
if [ -z "$PREFIX" ]; then
  if [ -w /usr/local/bin ] 2>/dev/null || [ -n "${SUDO_USER:-}" ]; then
    PREFIX="/usr/local/bin"
  elif [ "$(id -u)" -eq 0 ]; then
    PREFIX="/usr/local/bin"
  else
    PREFIX="$HOME/.local/bin"
  fi
fi
mkdir -p "$PREFIX" 2>/dev/null || true

if [ "$UNINSTALL" -eq 1 ]; then
  for p in "$PREFIX/$BIN" "/usr/local/bin/$BIN" "$HOME/.local/bin/$BIN"; do
    if [ -f "$p" ]; then rm -f "$p" && ok "removed $p"; fi
  done
  exit 0
fi

# ---------------------------------------------------------------------------
# Resolve download URL from the latest GitHub release
# ---------------------------------------------------------------------------
resolve_download() {
  local api tag json url
  if [ "$VERSION" = "latest" ]; then
    api="https://api.github.com/repos/${REPO}/releases/latest"
  else
    api="https://api.github.com/repos/${REPO}/releases/tags/${VERSION}"
  fi

  json="$(curl -fsSL -H 'User-Agent: lintop-installer' "$api" 2>/dev/null || true)"

  if [ -n "$json" ]; then
    tag="$(printf '%s' "$json" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
    url="$(printf '%s' "$json" \
      | grep -oE '"browser_download_url": *"[^"]+"' \
      | sed -E 's/.*"(https:[^"]+)".*/\1/' \
      | grep -Ev 'sha256|checksums|\.sig$' \
      | grep -E "linux.*(${ARCH}|${ARCH_ALT}).*(\.tar\.gz|\.tgz)$" \
      | head -n1)"
    if [ -n "$url" ]; then
      printf '%s\n' "$url"
      return 0
    fi
  fi

  # Fallback: construct the asset URL directly from the tag.
  if [ "$VERSION" = "latest" ]; then
    tag="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" 2>/dev/null | sed -E 's#.*/tag/##')" || true
  else
    tag="$VERSION"
  fi
  [ -n "${tag:-}" ] || return 1
  local ver="${tag#v}"
  url="https://github.com/${REPO}/releases/download/${tag}/${BIN}_${ver}_linux_${ARCH}.tar.gz"
  printf '%s\n' "$url"
}

# ---------------------------------------------------------------------------
# Install from a downloaded archive
# ---------------------------------------------------------------------------
install_from_release() {
  local url tmpdir
  url="$(resolve_download)" || return 1
  [ -n "$url" ] || return 1

  info "Downloading $(basename "$url")"
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  if ! curl -fsSL -H 'User-Agent: lintop-installer' "$url" -o "$tmpdir/pkg"; then
    return 1
  fi

  case "$url" in
    *.tar.gz|*.tgz)
      tar -xzf "$tmpdir/pkg" -C "$tmpdir" 2>/dev/null || return 1
      ;;
    *) mv "$tmpdir/pkg" "$tmpdir/$BIN" ;;
  esac

  local found
  found="$(find "$tmpdir" -type f -name "$BIN" -perm -u+x 2>/dev/null | head -n1)"
  [ -n "$found" ] || found="$(find "$tmpdir" -type f -name "$BIN" 2>/dev/null | head -n1)"
  [ -n "$found" ] || return 1

  install -m 0755 "$found" "$PREFIX/$BIN" 2>/dev/null || {
    cp "$found" "$PREFIX/$BIN" && chmod 0755 "$PREFIX/$BIN"
  }
  return 0
}

# ---------------------------------------------------------------------------
# Install from source (needs git + go)
# ---------------------------------------------------------------------------
install_from_source() {
  need git || die "git is required for --from-source"
  need go  || die "Go is required for --from-source (https://go.dev/dl/)"
  local tmpdir
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT
  info "Cloning ${REPO} and building from source"
  git clone --depth 1 "https://github.com/${REPO}.git" "$tmpdir/src" >/dev/null 2>&1
  ( cd "$tmpdir/src" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$tmpdir/$BIN" . )
  install -m 0755 "$tmpdir/$BIN" "$PREFIX/$BIN" 2>/dev/null || {
    cp "$tmpdir/$BIN" "$PREFIX/$BIN" && chmod 0755 "$PREFIX/$BIN"
  }
}

# ---------------------------------------------------------------------------
# Run
# ---------------------------------------------------------------------------
info "Installing ${BIN} (${ARCH}) to ${PREFIX}"

if [ "$FROM_SOURCE" -eq 0 ]; then
  if ! install_from_release; then
    warn "No prebuilt release available yet; falling back to a source build"
    install_from_source
  fi
else
  install_from_source
fi

ok "Installed $("$PREFIX/$BIN" --version 2>/dev/null || echo "$BIN") to $PREFIX/$BIN"

case ":$PATH:" in
  *":$PREFIX:"*) ;;
  *) warn "$PREFIX is not in your PATH. Add this to your shell profile:"
     printf '    export PATH="%s:$PATH"\n' "$PREFIX" ;;
esac

printf '\nRun it with:  %s\n\n' "${C_BOLD}${BIN}${C_RESET}"
