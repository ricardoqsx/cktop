#!/bin/sh

set -eu

DTOP_VERSION='@DTOP_VERSION@'
DTOP_REPOSITORY='ricardoqsx/cktop'
MINISIGN_PUBLIC_KEY='@MINISIGN_PUBLIC_KEY@'
SHA_DARWIN_AMD64='@SHA_DARWIN_AMD64@'
SHA_DARWIN_ARM64='@SHA_DARWIN_ARM64@'
SHA_LINUX_AMD64='@SHA_LINUX_AMD64@'
SHA_LINUX_ARM64='@SHA_LINUX_ARM64@'

scope=''
bin_dir=''
assume_yes=0
temporary=''

usage() {
    cat <<EOF
Usage: install-dtop.sh [options]

Install dtop ${DTOP_VERSION} and its configuration template.

Options:
  --scope user|system  installation scope (default: ask; user with --yes)
  --bin-dir PATH       binary destination directory (default depends on scope)
  --yes                non-interactive installation using defaults
  --help               print this help and exit
EOF
}

fail() {
    printf 'dtop installer: %s\n' "$*" >&2
    exit 1
}

cleanup() {
    if [ -n "$temporary" ] && [ -d "$temporary" ]; then
        rm -rf "$temporary"
    fi
}
trap cleanup 0 1 2 15

while [ "$#" -gt 0 ]; do
    case "$1" in
        --scope)
            [ "$#" -ge 2 ] || fail '--scope requires user or system'
            scope=$2
            shift 2
            ;;
        --bin-dir)
            [ "$#" -ge 2 ] || fail '--bin-dir requires a path'
            bin_dir=$2
            shift 2
            ;;
        --yes)
            assume_yes=1
            shift
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            fail "unknown option: $1"
            ;;
    esac
done

case "$scope" in
    ''|user|system) ;;
    *) fail '--scope must be user or system' ;;
esac
[ "$assume_yes" -eq 1 ] || [ -r /dev/tty ] || fail 'interactive input is unavailable; use --yes and --scope'

ask() {
    [ -r /dev/tty ] || fail 'interactive input is unavailable; use --yes and --scope'
    printf '%s' "$1" > /dev/tty
    IFS= read -r answer < /dev/tty || fail 'could not read interactive input'
    printf '%s' "$answer"
}

if [ -z "$scope" ]; then
    if [ "$assume_yes" -eq 1 ]; then
        scope=user
    else
        printf '\nInstall dtop %s for:\n' "$DTOP_VERSION" > /dev/tty
        printf '  1. Current user\n  2. All users\n  3. Cancel\n' > /dev/tty
        selection=$(ask 'Selection [1]: ')
        case "$selection" in
            ''|1) scope=user ;;
            2) scope=system ;;
            3) exit 0 ;;
            *) fail 'invalid installation scope' ;;
        esac
    fi
fi

if [ "$scope" = user ]; then
    [ -n "${HOME:-}" ] || fail 'HOME is not set'
    default_bin_dir=$HOME/.local/bin
    config_home=${XDG_CONFIG_HOME:-$HOME/.config}
    case "$config_home" in
        /*) ;;
        *) fail 'XDG_CONFIG_HOME must be an absolute path' ;;
    esac
    config_path=$config_home/dtop/dtop.conf
else
    default_bin_dir=/usr/local/bin
    config_path=/etc/dtop/dtop.conf
fi

if [ -z "$bin_dir" ]; then
    bin_dir=$default_bin_dir
    if [ "$assume_yes" -eq 0 ]; then
        selected_bin_dir=$(ask "Binary directory [$default_bin_dir]: ")
        if [ -n "$selected_bin_dir" ]; then
            bin_dir=$selected_bin_dir
        fi
    fi
fi
[ -n "$bin_dir" ] || fail 'binary directory cannot be empty'
case "$bin_dir" in
    /*) ;;
    *) fail 'binary directory must be an absolute path' ;;
esac

if [ "$assume_yes" -eq 0 ]; then
    printf '\nBinary: %s/dtop\nConfig: %s\n' "$bin_dir" "$config_path" > /dev/tty
    confirmation=$(ask 'Continue? [Y/n]: ')
    case "$confirmation" in
        ''|y|Y|yes|YES) ;;
        *) exit 0 ;;
    esac
fi

case "$(uname -s)" in
    Darwin) target_os=darwin ;;
    Linux) target_os=linux ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
esac
case "$(uname -m)" in
    x86_64|amd64) target_arch=amd64 ;;
    arm64|aarch64) target_arch=arm64 ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
esac

case "${target_os}_${target_arch}" in
    darwin_amd64) expected_sha256=$SHA_DARWIN_AMD64 ;;
    darwin_arm64) expected_sha256=$SHA_DARWIN_ARM64 ;;
    linux_amd64) expected_sha256=$SHA_LINUX_AMD64 ;;
    linux_arm64) expected_sha256=$SHA_LINUX_ARM64 ;;
    *) fail "unsupported platform: ${target_os}/${target_arch}" ;;
esac
case "$expected_sha256" in
    [0-9a-f][0-9a-f]*) ;;
    *) fail 'installer does not contain a valid artifact checksum' ;;
esac
[ "${#expected_sha256}" -eq 64 ] || fail 'installer checksum has an invalid length'

asset="dtop_${DTOP_VERSION}_${target_os}_${target_arch}.tar.gz"
release_base=${DTOP_RELEASE_BASE_URL:-"https://github.com/${DTOP_REPOSITORY}/releases/download/dtop-v${DTOP_VERSION}"}
temporary=$(mktemp -d "${TMPDIR:-/tmp}/dtop-install.XXXXXX") || fail 'could not create temporary directory'
archive=$temporary/$asset

download() {
    source_url=$1
    destination=$2
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --retry 3 --proto '=https,file' "$source_url" -o "$destination"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$source_url" -O "$destination"
    else
        fail 'curl or wget is required'
    fi
}

printf 'Downloading %s...\n' "$asset"
download "$release_base/$asset" "$archive"
if command -v sha256sum >/dev/null 2>&1; then
    actual_sha256=$(sha256sum "$archive" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    actual_sha256=$(shasum -a 256 "$archive" | awk '{print $1}')
else
    fail 'sha256sum or shasum is required'
fi
[ "$actual_sha256" = "$expected_sha256" ] || fail "checksum verification failed for $asset"

if command -v minisign >/dev/null 2>&1 && [ -n "$MINISIGN_PUBLIC_KEY" ]; then
    checksums=$temporary/SHA256SUMS
    signature=$temporary/SHA256SUMS.minisig
    download "$release_base/SHA256SUMS" "$checksums"
    download "$release_base/SHA256SUMS.minisig" "$signature"
    minisign -V -q -P "$MINISIGN_PUBLIC_KEY" -m "$checksums" -x "$signature" || fail 'Minisign verification failed'
    signed_sha256=$(awk -v asset="$asset" '$2 == asset { print $1 }' "$checksums")
    [ "$signed_sha256" = "$expected_sha256" ] || fail 'signed checksum does not match the installer'
fi

package_dir=$temporary/package
mkdir "$package_dir"
tar -xzf "$archive" -C "$package_dir"
[ -f "$package_dir/dtop" ] || fail 'archive does not contain dtop'
[ -f "$package_dir/dtop.conf.example" ] || fail 'archive does not contain dtop.conf.example'
chmod 0755 "$package_dir/dtop"
"$package_dir/dtop" --version >/dev/null || fail 'downloaded dtop binary could not be executed'

install_user_files() {
    mkdir -p "$bin_dir" "$(dirname "$config_path")"
    binary_temporary=$bin_dir/.dtop.install.$$
    install -m 0755 "$package_dir/dtop" "$binary_temporary"
    install -m 0644 "$package_dir/dtop.conf.example" "$config_path.example"
    if [ ! -e "$config_path" ]; then
        install -m 0600 "$package_dir/dtop.conf.example" "$config_path"
        config_result=created
    else
        config_result=preserved
    fi
    mv "$binary_temporary" "$bin_dir/dtop"
}

as_root() {
    if [ "$(id -u)" -eq 0 ]; then
        "$@"
    else
        command -v sudo >/dev/null 2>&1 || fail 'sudo is required for a system installation'
        sudo "$@"
    fi
}

install_system_files() {
    binary_temporary=$bin_dir/.dtop.install.$$
    as_root mkdir -p "$bin_dir" "$(dirname "$config_path")"
    as_root install -m 0755 "$package_dir/dtop" "$binary_temporary"
    as_root install -m 0644 "$package_dir/dtop.conf.example" "$config_path.example"
    if ! as_root test -e "$config_path"; then
        as_root install -m 0644 "$package_dir/dtop.conf.example" "$config_path"
        config_result=created
    else
        config_result=preserved
    fi
    as_root mv "$binary_temporary" "$bin_dir/dtop"
}

if [ "$scope" = user ]; then
    install_user_files
else
    install_system_files
fi

printf '\ndtop %s installed at %s/dtop\n' "$DTOP_VERSION" "$bin_dir"
printf 'Configuration %s: %s\n' "$config_result" "$config_path"
printf 'Configuration reference: %s.example\n' "$config_path"
case ":${PATH:-}:" in
    *:"$bin_dir":*) ;;
    *) printf 'Add %s to PATH before running dtop.\n' "$bin_dir" ;;
esac
