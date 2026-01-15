# Changelog

All notable changes to this project will be documented in this file.

## v0.1.3

- Add `--version` flag
- Tighten README usage and Windows install instructions

## v0.1.4

- Make `p test` use current proxy env only (no auto-detect)
- Installer uses fixed release URLs and auto-updates `VERSION` during release
- Refresh shell wrapper detection to avoid `eval` for `test`/`set`/`--version`

## v0.1.2

- Fix help handling for `-h/--help` and keep clipboard messages off stdout
- Update installer shell function to avoid `eval` for `status/detect/help`

## v0.1.1

- Add `install.sh` to simplify installation and shell setup
- Update README with one-line install example
- Include `install.sh` and README in release archives

## v0.1.0

- Add cross-platform proxy detection with env/system/app/port fallbacks
- Support common proxy apps (Clash family, Surge, Shadowsocks, V2RayN, sing-box, v2ray/xray)
- Default command copies proxy env script to clipboard for quick paste/apply
- Add `p on`, `p off`, `p status`, `p detect`, and shell targeting
- Provide tests for parsers and shell output
