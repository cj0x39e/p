# Repository Guidelines

## Project Structure & Module Organization
- `main.go` contains the CLI entrypoint and proxy detection logic.
- `main_test.go` holds unit tests for parsing and rendering helpers.
- `install.sh` installs the built binary and adds a shell wrapper.
- `release.sh` builds multi-platform archives and updates `CHANGELOG.md`.
- `README.md` documents usage and install paths.

## Build, Test, and Development Commands
- `go build -o p` builds the CLI binary locally.
- `go run .` runs the CLI from source (useful for quick checks).
- `go test ./...` runs all Go tests.
- `./install.sh /path/to/p` installs the compiled binary into your shell path.
- `./release.sh vX.Y.Z` builds release archives, tags the version, and updates changelog.

## Coding Style & Naming Conventions
- Use standard Go formatting (`gofmt`) and idiomatic Go style.
- Indentation follows Go defaults (tabs for blocks, spaces for alignment).
- Exported identifiers use `PascalCase`; internal helpers use `camelCase`.
- Test files are named `*_test.go` with `TestXxx` functions.

## Testing Guidelines
- Tests use Go’s built-in `testing` package in `main_test.go`.
- Prefer table-driven tests for new parsing logic.
- Run `go test ./...` before pushing changes.

## Commit & Pull Request Guidelines
- Commit messages are short, imperative, and capitalized (e.g., "Fix help output").
- Keep commits focused and update `README.md` or `CHANGELOG.md` when behavior changes.
- PRs should include: a brief summary, rationale, and any usage examples if CLI output changes.

## Configuration & Security Tips
- Proxy detection reads environment variables and local config files; avoid committing real proxy URLs.
- If adding new config sources, document the lookup order in `README.md`.
