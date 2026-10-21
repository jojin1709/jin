#!/usr/bin/env bash
# Install Jin from the latest release.
#
#     curl -fsSL https://raw.githubusercontent.com/jojin1709/jin/main/install.sh | sh
#
# It works out the operating system and the processor, downloads the matching
# archive from the newest release, checks it against the published checksum,
# and puts the jin binary in ~/.local/bin.
#
# Set JIN_INSTALL_DIR to install somewhere else.

set -euo pipefail

REPO="jojin1709/jin"
BIN_DIR="${JIN_INSTALL_DIR:-$HOME/.local/bin}"

die() {
    echo "install.sh: $*" >&2
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || die "$2"
}

need curl "curl is needed to download Jin. Install curl and try again."
need tar "tar is needed to unpack the download. Install tar and try again."

# ---------------------------------------------------------------- the machine

os="$(uname -s)"
case "$os" in
    Linux)  os=linux ;;
    Darwin) os=darwin ;;
    *)
        die "$os is not supported. The releases are for Linux and macOS. On Windows, use install.ps1."
        ;;
esac

arch="$(uname -m)"
case "$arch" in
    x86_64|amd64)  arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *)
        die "a $arch processor is not supported. The releases are for amd64 and arm64."
        ;;
esac

# ------------------------------------------------------------- the release

latest="https://api.github.com/repos/$REPO/releases/latest"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

status="$(curl -sSL -o "$tmp/latest.json" -w '%{http_code}' "$latest" 2>/dev/null)" \
    || die "could not reach GitHub to find the latest release."

if [ "$status" = "404" ]; then
    die "there is no release to install yet. The releases are at https://github.com/$REPO/releases"
fi

if [ "$status" != "200" ]; then
    die "GitHub answered $status when asked for the latest release, so nothing was installed."
fi

tag="$(grep -o '"tag_name":[[:space:]]*"[^"]*"' "$tmp/latest.json" \
    | head -n 1 \
    | sed 's/.*"\(.*\)"$/\1/')"

if [ -z "$tag" ]; then
    die "there is no release to install yet. The releases are at https://github.com/$REPO/releases"
fi

name="jin_${tag}_${os}_${arch}"
if [ "$os" = "windows" ]; then
    archive="$name.zip"
else
    archive="$name.tar.gz"
fi

base="https://github.com/$REPO/releases/download/$tag"

# ---------------------------------------------------------------- download

echo "downloading jin $tag for $os/$arch..."

curl -fsSL -o "$tmp/$archive" "$base/$archive" \
    || die "could not download $archive from $base"

curl -fsSL -o "$tmp/SHA256SUMS.txt" "$base/SHA256SUMS.txt" \
    || die "could not download the checksums from $base"

# ---------------------------------------------------------------- checksum

sha256_of() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

expected="$(awk -v file="$archive" '$2 == file || $2 == "*" file { print $1; exit }' "$tmp/SHA256SUMS.txt")"

if [ -z "$expected" ]; then
    die "the checksum file does not mention $archive, so the download cannot be checked."
fi

actual="$(sha256_of "$tmp/$archive")"

if [ "$expected" != "$actual" ]; then
    die "the checksum does not match. The download may be damaged, so nothing was installed."
fi

# ----------------------------------------------------------------- install

if [ "$os" = "windows" ]; then
    need unzip "unzip is needed to unpack the download. Install unzip and try again."
    (cd "$tmp" && unzip -q "$archive")
else
    tar -xzf "$tmp/$archive" -C "$tmp"
fi

binary="$(find "$tmp" -type f -name 'jin' | head -n 1)"
if [ -z "$binary" ]; then
    die "the archive did not contain a jin binary."
fi

mkdir -p "$BIN_DIR"
cp "$binary" "$BIN_DIR/jin"
chmod 0755 "$BIN_DIR/jin"

# ------------------------------------------------------------------ report

echo
"$BIN_DIR/jin" --version
echo "installed to $BIN_DIR/jin"

case ":$PATH:" in
    *":$BIN_DIR:"*)
        ;;
    *)
        echo
        echo "$BIN_DIR is not on your PATH. Add it with:"
        echo
        echo "    export PATH=\"$BIN_DIR:\$PATH\""
        echo
        echo "Put that line in your shell profile to make it stick."
        ;;
esac
