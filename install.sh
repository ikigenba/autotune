#!/bin/sh
set -eu

REPO="ikigenba/autotune"
BINARY="autotune"

fail() {
    echo "$BINARY: $*" >&2
    exit 1
}

case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
esac

bindir=${BINDIR:-${PREFIX:-$HOME/.local}/bin}
version=${AUTOTUNE_VERSION:-latest}
if [ "$version" = latest ]; then
    url="https://github.com/$REPO/releases/latest/download/${BINARY}_${os}_${arch}.tar.gz"
else
    url="https://github.com/$REPO/releases/download/$version/${BINARY}_${os}_${arch}.tar.gz"
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT INT TERM
archive="$tmpdir/$BINARY.tar.gz"

command -v curl >/dev/null 2>&1 || fail "curl is required"
curl -fsSL "$url" -o "$archive"
tar -xzf "$archive" -C "$tmpdir"
install -d "$bindir"
install -m 0755 "$tmpdir/$BINARY" "$bindir/$BINARY"
"$bindir/$BINARY" -V

case ":$PATH:" in
    *":$bindir:"*) ;;
    *) echo "warning: $bindir is not on PATH" >&2 ;;
esac
