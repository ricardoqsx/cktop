#!/bin/sh

set -eu

[ "$#" -eq 5 ] || {
    printf 'usage: %s VERSION SHA256SUMS MINISIGN_PUBLIC_KEY TEMPLATE OUTPUT\n' "$0" >&2
    exit 2
}

version=$1
checksums=$2
public_key=$3
template=$4
output=$5

printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || {
    printf 'invalid stable version: %s\n' "$version" >&2
    exit 1
}

checksum() {
    asset=$1
    value=$(awk -v asset="$asset" '$2 == asset { print $1 }' "$checksums")
    case "$value" in
        *[!0-9a-f]*|'') valid=0 ;;
        *) valid=1 ;;
    esac
    [ "$valid" -eq 1 ] && [ "${#value}" -eq 64 ] || {
        printf 'missing checksum for %s\n' "$asset" >&2
        exit 1
    }
    printf '%s' "$value"
}

darwin_amd64=$(checksum "dtop_${version}_darwin_amd64.tar.gz")
darwin_arm64=$(checksum "dtop_${version}_darwin_arm64.tar.gz")
linux_amd64=$(checksum "dtop_${version}_linux_amd64.tar.gz")
linux_arm64=$(checksum "dtop_${version}_linux_arm64.tar.gz")

case "$public_key" in
    RW*) ;;
    *) printf 'invalid Minisign public key\n' >&2; exit 1 ;;
esac
case "$public_key" in
    *[!A-Za-z0-9+/=]*) printf 'invalid Minisign public key\n' >&2; exit 1 ;;
esac
[ "${#public_key}" -eq 56 ] || { printf 'invalid Minisign public key length\n' >&2; exit 1; }

temporary=$output.tmp
sed \
    -e "s|@DTOP_VERSION@|$version|g" \
    -e "s|@MINISIGN_PUBLIC_KEY@|$public_key|g" \
    -e "s|@SHA_DARWIN_AMD64@|$darwin_amd64|g" \
    -e "s|@SHA_DARWIN_ARM64@|$darwin_arm64|g" \
    -e "s|@SHA_LINUX_AMD64@|$linux_amd64|g" \
    -e "s|@SHA_LINUX_ARM64@|$linux_arm64|g" \
    "$template" > "$temporary"
chmod 0755 "$temporary"
mv "$temporary" "$output"
