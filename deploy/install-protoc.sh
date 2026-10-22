#!/bin/sh
# Install a checksum-verified protoc archive without relying on one download host.
set -eu

version=${1:-}
install_prefix=${2:-/usr/local}
case "${version}:$(dpkg --print-architecture)" in
    35.1:amd64)
        archive_arch=x86_64
        expected_sha256=6930ebf62bd4ea607b98fff052596c6ee564b9835b4ce172c75a3f53ae9d91b7
        ;;
    35.1:arm64)
        archive_arch=aarch_64
        expected_sha256=01bf9d08808c7f96678b63f4bd8efa559bb4f83d5a7a270d5edaf507f9d5d9cf
        ;;
    *)
        echo "Unsupported protoc version/architecture: ${version}:$(dpkg --print-architecture). Add its official release SHA-256 before upgrading." >&2
        exit 2
        ;;
esac

filename="protoc-${version}-linux-${archive_arch}.zip"
archive=$(mktemp /tmp/protoc.XXXXXXXX.zip)
trap 'rm -f "$archive"' EXIT HUP INT TERM

fetch() {
    url=$1
    rm -f "$archive"
    curl --fail --location --show-error --silent \
        --proto '=https' --proto-redir '=https' \
        --connect-timeout 12 --max-time 120 \
        --retry 1 --retry-all-errors --retry-delay 1 \
        --output "$archive" "$url"
}

# The github.com Release URL redirects to release-assets.githubusercontent.com,
# which can fail TLS negotiation on proxy-restricted networks. SourceForge is
# an unaffiliated exact release mirror; the official SHA-256 gates both paths.
github_url="https://github.com/protocolbuffers/protobuf/releases/download/v${version}/${filename}"
mirror_url="https://downloads.sourceforge.net/project/protocol-buffers.mirror/v${version}/${filename}"
if fetch "$github_url"; then
    echo 'protoc: downloaded from GitHub Release'
elif fetch "$mirror_url"; then
    echo 'protoc: downloaded from SourceForge mirror'
else
    echo 'protoc: GitHub Release and SourceForge mirror are both unavailable; check Mihomo TLS routing.' >&2
    exit 1
fi

printf '%s  %s\n' "$expected_sha256" "$archive" | sha256sum --check --status || {
    echo 'protoc: downloaded archive does not match the official release SHA-256; refusing installation.' >&2
    exit 1
}
unzip -q "$archive" -d "$install_prefix"
chmod +x "$install_prefix/bin/protoc"
[ "$("$install_prefix/bin/protoc" --version)" = "libprotoc ${version}" ] || {
    echo 'protoc: installed compiler version does not match the requested release.' >&2
    exit 1
}
echo "protoc: installed verified ${version}"
