# p

`p` is a cross-platform proxy helper that detects local proxy settings and outputs shell commands to apply them.

## Install

Quick install script (downloads latest release):

```sh
curl -fsSL https://raw.githubusercontent.com/cj0x39e/p/main/install.sh | sh
```

Install to a custom directory:

```sh
P_INSTALL_DIR="$HOME/.local/bin" curl -fsSL https://raw.githubusercontent.com/cj0x39e/p/main/install.sh | sh
```

From a local build:

```sh
go build -o p
./install.sh ./p
```

Windows (PowerShell, latest release):

```powershell
$repo = "cj0x39e/p"
$asset = "p_latest_windows_amd64.zip"
$url = "https://github.com/$repo/releases/latest/download/$asset"
Invoke-WebRequest $url -OutFile $asset
Expand-Archive $asset -DestinationPath ".\\p_latest_windows_amd64"
Copy-Item ".\\p_latest_windows_amd64\\p.exe" "$env:USERPROFILE\\bin\\p.exe" -Force
```

## Usage

Default behavior copies the proxy env script to your clipboard. Paste and run it in your shell.

```sh
p
```

Common commands:

| Command | Purpose |
| --- | --- |
| `p` | copy proxy env commands to clipboard |
| `p on` | print shell commands to enable proxy |
| `p off` | print shell commands to disable proxy |
| `p status` | show detected proxy |
| `p detect` | show detection details |
| `p test` | test proxy connectivity with curl |
| `p set 7890` | save local HTTP proxy port |
| `p --shell sh` | force output shell (`sh`, `fish`, `ps`) |
| `p --version` | print version |

## Build from Source

```sh
go build -o p
```

## Detection Order

1. Environment variables
2. System proxy settings (macOS/Windows/GNOME)
3. App configs (Clash family, Surge, Shadowsocks, V2RayN, sing-box, v2ray/xray)
4. Common local ports

## Notes

- `ping` uses ICMP and does not use HTTP/SOCKS proxies. Use `curl` to verify.
- To apply in the current shell without clipboard, use `eval "$(p on)"`.
- If no proxy is detected, `p` will prompt for a local HTTP proxy port and save it to the user config (for example `~/.config/p/config.json`).
