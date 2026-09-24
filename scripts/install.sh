#!/bin/sh
# Install hcl-linter from GitHub Releases, verifying the archive checksum.
#
#   curl -fsSL https://raw.githubusercontent.com/bard-works/hcl-linter/main/scripts/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- -v 0.1.0 -b ~/.local/bin
#
# Options:
#   -v VERSION   version to install (default: latest release; "v" prefix optional)
#   -b DIR       install directory (default: ~/.local/bin)
#
# HCL_LINTER_DOWNLOAD_URL overrides the release download base URL (testing,
# mirrors); the version-specific path segment is appended to it.
set -eu

repo="bard-works/hcl-linter"
version="latest"
bindir="${HOME}/.local/bin"

while getopts "v:b:h" opt; do
  case "$opt" in
    v) version="$OPTARG" ;;
    b) bindir="$OPTARG" ;;
    h) echo "usage: install.sh [-v VERSION] [-b DIR]"; exit 0 ;;
    *) exit 2 ;;
  esac
done

fail() { echo "install.sh: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || fail "$1 is required"; }
need curl
need uname

if [ "$version" = "latest" ]; then
  # The /releases/latest redirect avoids the rate-limited API.
  latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${repo}/releases/latest") ||
    fail "could not resolve the latest release"
  version="${latest_url##*/}"
fi
version="${version#v}"

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  MINGW* | MSYS* | CYGWIN* | Windows_NT) os=windows ;;
  *) fail "unsupported OS: $(uname -s)" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "unsupported architecture: $(uname -m)" ;;
esac

ext="tar.gz"
exe="hcl-linter"
if [ "$os" = "windows" ]; then
  ext="zip"
  exe="hcl-linter.exe"
fi

archive="hcl-linter_${version}_${os}_${arch}.${ext}"
base="${HCL_LINTER_DOWNLOAD_URL:-https://github.com/${repo}/releases/download}/v${version}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "Downloading ${archive}..." >&2
curl -fsSL -o "${tmp}/${archive}" "${base}/${archive}" || fail "download failed: ${base}/${archive}"
curl -fsSL -o "${tmp}/checksums.txt" "${base}/checksums.txt" || fail "download failed: ${base}/checksums.txt"

expected=$(awk -v f="$archive" '$2 == f { print $1 }' "${tmp}/checksums.txt")
[ -n "$expected" ] || fail "${archive} is not listed in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "${tmp}/${archive}" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "${tmp}/${archive}" | awk '{ print $1 }')
else
  fail "sha256sum or shasum is required to verify the download"
fi
[ "$expected" = "$actual" ] || fail "checksum mismatch for ${archive}"

if [ "$ext" = "zip" ]; then
  need unzip
  unzip -q "${tmp}/${archive}" -d "${tmp}/x"
else
  mkdir -p "${tmp}/x"
  tar -xzf "${tmp}/${archive}" -C "${tmp}/x"
fi

mkdir -p "$bindir"
mv "${tmp}/x/${exe}" "${bindir}/${exe}"
chmod +x "${bindir}/${exe}"
echo "Installed hcl-linter ${version} to ${bindir}/${exe}" >&2
case ":${PATH}:" in
  *":${bindir}:"*) ;;
  *) echo "Note: ${bindir} is not on PATH" >&2 ;;
esac
