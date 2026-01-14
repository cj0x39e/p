#!/usr/bin/env sh
set -eu

BIN_SRC=${1:-"./p"}
if [ ! -f "$BIN_SRC" ]; then
  echo "Binary not found: $BIN_SRC" >&2
  echo "Usage: ./install.sh /path/to/p" >&2
  exit 1
fi

BIN_DIR="/usr/local/bin"
if [ ! -w "$BIN_DIR" ]; then
  BIN_DIR="$HOME/.local/bin"
fi

mkdir -p "$BIN_DIR"
install -m 0755 "$BIN_SRC" "$BIN_DIR/p"

SHELL_NAME=$(basename "${SHELL:-sh}")
RC_FILE=""
FUNC_LINE_SH='p() { eval "$(command p "$@")"; }'
FUNC_LINE_FISH='function p; eval (command p $argv); end'

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
  if [ ! -f "$RC_FILE" ] || ! grep -q 'p() { eval "$(command p' "$RC_FILE"; then
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
