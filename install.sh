#!/usr/bin/env sh
set -eu

REPO="cj0x39e/p"
VERSION="v0.1.3"
BIN_SRC=""

if [ $# -gt 0 ]; then
  BIN_SRC="$1"
elif [ -f "./p" ]; then
  BIN_SRC="./p"
fi

download_release() {
  if ! command -v curl >/dev/null 2>&1; then
    echo "curl is required to download releases." >&2
    exit 1
  fi

  uname_s=$(uname -s | tr '[:upper:]' '[:lower:]')
  uname_m=$(uname -m)
  case "$uname_s" in
    darwin) goos="darwin" ;;
    linux) goos="linux" ;;
    *) echo "Unsupported OS: $uname_s" >&2; exit 1 ;;
  esac
  case "$uname_m" in
    x86_64|amd64) goarch="amd64" ;;
    arm64|aarch64) goarch="arm64" ;;
    *) echo "Unsupported arch: $uname_m" >&2; exit 1 ;;
  esac

  filename="p_${VERSION#v}_${goos}_${goarch}.tar.gz"
  url="https://github.com/$REPO/releases/download/$VERSION/$filename"
  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  echo "Downloading $url"
  curl -fsSL -o "$tmpdir/$filename" "$url"
  (cd "$tmpdir" && tar -xzf "$filename")
  BIN_SRC="$tmpdir/p_${VERSION#v}_${goos}_${goarch}/p"
  if [ ! -f "$BIN_SRC" ]; then
    echo "Downloaded binary not found." >&2
    exit 1
  fi
}

if [ -z "$BIN_SRC" ]; then
  download_release
fi

if [ ! -f "$BIN_SRC" ]; then
  echo "Binary not found: $BIN_SRC" >&2
  echo "Usage: ./install.sh /path/to/p" >&2
  exit 1
fi

BIN_DIR=${P_INSTALL_DIR:-"/usr/local/bin"}
if [ -z "${P_INSTALL_DIR:-}" ] && [ ! -w "$BIN_DIR" ]; then
  BIN_DIR="$HOME/.local/bin"
fi

mkdir -p "$BIN_DIR"
install -m 0755 "$BIN_SRC" "$BIN_DIR/p"

SHELL_NAME=$(basename "${SHELL:-sh}")
RC_FILE=""
FUNC_LINE_SH='p() { case "$1" in status|detect|help|-h|--help|test|set|--version|-v|version) command p "$@";; *) eval "$(command p "$@")";; esac; }'
FUNC_LINE_FISH='function p; switch $argv[1]; case status detect help -h --help test set --version -v version; command p $argv; case "*"; eval (command p $argv); end; end'

case "$SHELL_NAME" in
  zsh) RC_FILE="$HOME/.zshrc" ;;
  bash) RC_FILE="$HOME/.bashrc" ;;
  fish) RC_FILE="$HOME/.config/fish/config.fish" ;;
  *) RC_FILE="$HOME/.profile" ;;
esac

if [ "$SHELL_NAME" = "fish" ]; then
  if [ ! -f "$RC_FILE" ] || ! grep -q 'function p; eval (command p' "$RC_FILE"; then
    printf "\n%s\n" "$FUNC_LINE_FISH" >> "$RC_FILE"
  fi
else
  if [ ! -f "$RC_FILE" ] || ! grep -q 'p() { case "\\$1" in status' "$RC_FILE"; then
    printf "\n%s\n" "$FUNC_LINE_SH" >> "$RC_FILE"
  fi
fi

if [ "$BIN_DIR" = "$HOME/.local/bin" ]; then
  PATH_LINE='export PATH="$HOME/.local/bin:$PATH"'
  if [ ! -f "$RC_FILE" ] || ! grep -q "$HOME/.local/bin" "$RC_FILE"; then
    printf "\n%s\n" "$PATH_LINE" >> "$RC_FILE"
  fi
fi

echo "Installed p to $BIN_DIR/p"
echo "Updated shell config: $RC_FILE"
echo "Open a new terminal or run: source \"$RC_FILE\""
