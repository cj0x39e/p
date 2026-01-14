# p

`p` is a cross-platform proxy helper that detects local proxy settings and outputs shell commands to apply them.

## Install

Build from source:

```sh
go build -o p
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

Force shell output:

```sh
p --shell sh
p --shell fish
p --shell ps
```

## Detection Order

1. Environment variables
2. System proxy settings (macOS/Windows/GNOME)
3. App configs (Clash family, Surge, Shadowsocks, V2RayN, sing-box, v2ray/xray)
4. Common local ports

## Notes

- `ping` uses ICMP and does not use HTTP/SOCKS proxies. Use `curl` to verify.
- To apply in the current shell without clipboard, use `eval "$(p on)"`.
