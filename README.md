# p

`p` is a cross-platform proxy helper that detects local proxy settings and outputs shell commands to apply them.

## Install

Build from source:

```sh
go build -o p
```

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
$tag = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
$asset = "p_$($tag.TrimStart('v'))_windows_amd64.zip"
$url = "https://github.com/$repo/releases/download/$tag/$asset"
Invoke-WebRequest $url -OutFile $asset
Expand-Archive $asset -DestinationPath ".\\p_$($tag.TrimStart('v'))_windows_amd64"
Copy-Item ".\\p_$($tag.TrimStart('v'))_windows_amd64\\p.exe" "$env:USERPROFILE\\bin\\p.exe" -Force
```

## Usage

Default behavior copies the proxy env script to your clipboard. Paste and run it in your shell.

```sh
p
```

Explicitly print the script to stdout:

```sh
p on
```

Disable proxy:

```sh
p off
```

Show detected proxy:

```sh
p status
```

Test proxy connectivity with curl:

```sh
p test
```

Save a local HTTP proxy port to user config:

```sh
p set 7890
```

Force shell output:

```sh
p --shell sh
p --shell fish
p --shell ps
```

Print version:

```sh
p --version
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
