#!/usr/bin/env sh
set -eu

usage() {
  cat <<'EOF'
Usage: ./release.sh vX.Y.Z

Builds release archives for common targets, generates checksums, tags, and
pushes the tag. It also appends a template entry to CHANGELOG.md.
EOF
}

VERSION=${1:-}
if [ -z "$VERSION" ]; then
  usage
  exit 1
fi

case "$VERSION" in
  v*) ;;
  *)
    echo "Version must start with v, e.g. v0.1.2" >&2
    exit 1
    ;;
esac

if git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo "Tag already exists: $VERSION" >&2
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Working tree is dirty. Please commit first." >&2
  exit 1
fi

if [ -f install.sh ]; then
  tmpfile=$(mktemp)
  sed "s/^VERSION=\"v[^\"]*\"/VERSION=\"$VERSION\"/" install.sh > "$tmpfile"
  mv "$tmpfile" install.sh
  chmod +x install.sh
fi

DIST_DIR="dist"
mkdir -p "$DIST_DIR"

build() {
  goos=$1
  goarch=$2
  outdir="$DIST_DIR/p_${VERSION#v}_${goos}_${goarch}"
  binname=p
  if [ "$goos" = "windows" ]; then binname=p.exe; fi
  rm -rf "$outdir"
  mkdir -p "$outdir"
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -ldflags "-X main.version=$VERSION" -o "$outdir/$binname" ./
  cp -f install.sh README.md "$outdir/"
  if [ "$goos" = "windows" ]; then
    (cd "$DIST_DIR" && zip -qr "p_${VERSION#v}_${goos}_${goarch}.zip" "p_${VERSION#v}_${goos}_${goarch}")
  else
    (cd "$DIST_DIR" && tar -czf "p_${VERSION#v}_${goos}_${goarch}.tar.gz" "p_${VERSION#v}_${goos}_${goarch}")
  fi
}

build darwin arm64
build darwin amd64
build linux amd64
build windows amd64

if [ -f "$DIST_DIR/p_${VERSION#v}_darwin_arm64.tar.gz" ]; then
  cp -f "$DIST_DIR/p_${VERSION#v}_darwin_arm64.tar.gz" "$DIST_DIR/p_latest_darwin_arm64.tar.gz"
fi
if [ -f "$DIST_DIR/p_${VERSION#v}_darwin_amd64.tar.gz" ]; then
  cp -f "$DIST_DIR/p_${VERSION#v}_darwin_amd64.tar.gz" "$DIST_DIR/p_latest_darwin_amd64.tar.gz"
fi
if [ -f "$DIST_DIR/p_${VERSION#v}_linux_amd64.tar.gz" ]; then
  cp -f "$DIST_DIR/p_${VERSION#v}_linux_amd64.tar.gz" "$DIST_DIR/p_latest_linux_amd64.tar.gz"
fi
if [ -f "$DIST_DIR/p_${VERSION#v}_windows_amd64.zip" ]; then
  cp -f "$DIST_DIR/p_${VERSION#v}_windows_amd64.zip" "$DIST_DIR/p_latest_windows_amd64.zip"
fi

find "$DIST_DIR" -maxdepth 1 -type d -name "p_${VERSION#v}_*" -exec rm -rf {} +
find "$DIST_DIR" -maxdepth 1 -type f \( -name "p_${VERSION#v}_*" -o -name "p_latest_*" \) -print0 | xargs -0 shasum -a 256 > "$DIST_DIR/checksums.txt"

CHANGELOG="CHANGELOG.md"
if [ ! -f "$CHANGELOG" ]; then
  cat <<'EOF' > "$CHANGELOG"
# Changelog

All notable changes to this project will be documented in this file.
EOF
fi
if ! grep -q "^## $VERSION$" "$CHANGELOG"; then
  cat <<EOF >> "$CHANGELOG"

## $VERSION

- TODO: add release notes
EOF
fi

git tag -a "$VERSION" -m "$VERSION"
git push origin "$VERSION"

echo "Built artifacts in $DIST_DIR/"
echo "Changelog: $CHANGELOG"
echo "Next: create GitHub Release and upload artifacts."
