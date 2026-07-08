#!/bin/sh
# Installs the latest abstr release from GitHub Releases.
# Usage: curl -fsSL https://raw.githubusercontent.com/abstraction-dev/cli/main/install.sh | sh
set -eu

REPO="abstraction-dev/cli"
BINARY="abstr"

fail() {
	echo "error: $1" >&2
	exit 1
}

os="$(uname -s)"
case "$os" in
Darwin) os="darwin" ;;
Linux) os="linux" ;;
*)
	fail "unsupported OS: $os. Download a release manually from https://github.com/${REPO}/releases/latest"
	;;
esac

arch="$(uname -m)"
case "$arch" in
x86_64 | amd64) arch="amd64" ;;
arm64 | aarch64) arch="arm64" ;;
*)
	fail "unsupported architecture: $arch. Download a release manually from https://github.com/${REPO}/releases/latest"
	;;
esac

asset="${BINARY}_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/latest/download/${asset}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

echo "Downloading ${url}..." >&2
if ! curl -fsSL "$url" -o "${tmpdir}/${asset}"; then
	fail "download failed: ${url}"
fi

if ! tar -xzf "${tmpdir}/${asset}" -C "$tmpdir"; then
	fail "failed to extract ${asset}"
fi

if [ ! -f "${tmpdir}/${BINARY}" ]; then
	fail "extracted archive did not contain a '${BINARY}' binary"
fi
chmod +x "${tmpdir}/${BINARY}"

bindir="${BINDIR:-}"
if [ -z "$bindir" ]; then
	if [ -w /usr/local/bin ]; then
		bindir="/usr/local/bin"
	else
		bindir="${HOME}/.local/bin"
		mkdir -p "$bindir"
	fi
fi

mv "${tmpdir}/${BINARY}" "${bindir}/${BINARY}"
echo "Installed ${BINARY} to ${bindir}/${BINARY}" >&2

case ":${PATH}:" in
*":${bindir}:"*) ;;
*) echo "warning: ${bindir} is not on your PATH. Add it to your shell profile." >&2 ;;
esac

"${bindir}/${BINARY}" --version
