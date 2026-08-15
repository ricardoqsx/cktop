#!/bin/sh

set -eu

[ "$#" -eq 3 ] || {
    printf 'usage: %s VERSION DIST_DIR MINISIGN_PUBLIC_KEY\n' "$0" >&2
    exit 2
}

version=$1
dist_dir=$2
public_key=$3
temporary=$(mktemp -d "${TMPDIR:-/tmp}/dtop-installer-test.XXXXXX")
trap 'rm -rf "$temporary"' 0 1 2 15

installer=$temporary/install-dtop.sh
sh scripts/render-dtop-installer.sh "$version" "$dist_dir/SHA256SUMS" "$public_key" scripts/install-dtop.sh "$installer"

home=$temporary/home
config_home=$home/config
bin=$home/.local/bin/dtop
config=$config_home/dtop/dtop.conf
mkdir -p "$home"

run_installer() {
    HOME=$home \
    XDG_CONFIG_HOME=$config_home \
    DTOP_RELEASE_BASE_URL="file://$dist_dir" \
    sh "$installer" --yes --scope user
}

run_installer
[ -x "$bin" ] || { printf 'installer did not create dtop\n' >&2; exit 1; }
[ -f "$config" ] || { printf 'installer did not create config\n' >&2; exit 1; }
"$bin" --version | grep -F "dtop $version" >/dev/null

printf '\n# preserve-this-line\n' >> "$config"
run_installer
grep -F '# preserve-this-line' "$config" >/dev/null || {
    printf 'installer overwrote the existing config\n' >&2
    exit 1
}

printf 'installer smoke test passed\n'
