#!/bin/sh
set -e

fatal() { echo "fatal: $*" 1>&2; exit 1; }

usage() {
cat<<EOF
Syntax: $(basename "$0") [--options] destination
Download, verify and install wireleap client

Arguments:

    destination         destination path (eg. \$HOME/wireleap)

Options:

    --symlink=          symlink path (eg. \$HOME/.local/bin/wireleap)
    --skip-gpg          skip gpg verification (not recommended)
    --skip-checksum     skip checksum verification (not recommended)

EOF
exit 1
}

# builds@wireleap.com: 693C86E9DECA9D07D79FF9D22ECD72AD056012E1
PUBKEY="
-----BEGIN PGP PUBLIC KEY BLOCK-----

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
zfOfffOA5ohs0UtKFDx9+nQoOONGhplyBA==
=oKFY
-----END PGP PUBLIC KEY BLOCK-----
"

_verify_environment() {
    [ "$(id -u)" = "0" ] && fatal "should not be run as root"

    case "$(uname -s):$(uname -m)" in
        Linux:x86_64) ;;
        Darwin:x86_64) ;;
        *) fatal "unsupported system: $(uname -a)" ;;
    esac

    if [ "$symlink_path" ]; then
        symdir="$(dirname "$symlink_path")"
        symfile="$(basename "$symlink_path")"
        [ -d "$symlink_path" ] && fatal "$symlink_path is a directory"
        [ -e "$symlink_path" ] && fatal "$symlink_path already exists"
        [ -w "$symdir" ] || fatal "$symdir is not writable"
        _in_path "$symdir" || fatal "$symdir is not in \$PATH"
        command -v "$symfile" >/dev/null && fatal "$symfile already in \$PATH"
    fi

    command -v curl >/dev/null || fatal "curl not found"

    e="gpg not found. install or specify --skip-gpg (not recommended)"
    [ "$skip_gpg" ] || command -v gpg >/dev/null || fatal "$e"

    e="sha512sum not found. install or --skip-checksum (not recommended)"
    [ "$skip_checksum" ] || command -v sha512sum >/dev/null || fatal "$e"

    return 0
}

_download() {
    filename="$1"
    eval set -- "$2"
    dist="https://github.com/wireleap/client/releases/latest/download/"
    curl "$@" -O "$dist/$filename" || fatal "$filename download failed"
    return 0
}

_verify_gpg() {
    [ "$skip_gpg" ] && echo "  SKIPPED!" && return 0
    filename="$1"
    eval set -- "--no-default-keyring --keyring ./keyring.gpg"
    echo "$PUBKEY" | gpg "$@" --import > out.gpg 2>&1 || return 1
    gpg "$@" --verify "$filename" > out.gpg 2>&1 || return 1
    rm -f keyring.gpg keyring.gpg~ out.gpg
    return 0
}

_verify_checksum() {
    [ "$skip_checksum" ] && echo "  SKIPPED!" && return 0
    filename="$1"
    eval set -- "--check --status"
    sha512sum "$@" "$filename" || fatal "sha512sum $filename check failed"
    return 0
}

_in_path() {
    echo "$PATH" | grep -q "^$1:" && return 0
    echo "$PATH" | grep -q ":$1:" && return 0
    echo "$PATH" | grep -q ":$1$" && return 0
    return 1
}

main() {
    while [ "$1" != "" ]; do
        case "$1" in
            --help|-h|help)     usage ;;
            --symlink=*)        symlink_path="${1##*=}" ;;
            --skip-gpg)         skip_gpg="true" ;;
            --skip-checksum)    skip_checksum="true" ;;
            *)                  if [ -n "$d" ]; then usage; else d="$1"; fi ;;
        esac
        shift
    done

    [ -n "$d" ] || usage
    [ -d "$d" ] && fatal "$d already exists"
    binary="wireleap"
    binarydir="$(realpath "$d")"

    echo "* verifying environment ..."
    _verify_environment

    echo "* preparing quarantine ..."
    mkdir -p "$binarydir/quarantine"
    cd "$binarydir/quarantine"

    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    echo "* downloading $binary ..."
    _download "${binary}_$os-amd64"

    echo "* downloading $binary.hash ..."
    _download "${binary}_$os-amd64.hash" "--silent --show-error"

    echo "* performing gpg verification on $binary.hash ..."
    _verify_gpg "$binary.hash" || fatal "gpg error\n$(cat out.gpg)"

    echo "* performing checksum verification on $binary ..."
    _verify_checksum "$binary.hash"

    echo "* releasing $binary from quarantine ..."
    cd "$binarydir"
    mv "$binarydir/quarantine/$binary" "$binarydir/$binary"
    rm "$binarydir/quarantine/$binary.hash"
    rmdir "$binarydir/quarantine"

    echo "* setting $binary executable flag ..."
    chmod +x "$binarydir/$binary"

    echo "* performing $binary initialization ..."
    "$binarydir/$binary" init > out.init 2>&1 || fatal "error\n$(cat out.init)"
    rm -f out.init

    if [ "$symlink_path" ]; then
        echo "* creating symlink $symlink_path ..."
        ln -sf "$binarydir/$binary" "$symlink_path"
    fi

    echo "* complete"

    if [ -e "$binarydir/wireleap_tun" ]; then
        echo
        echo "To enable TUN support, execute the following commands:"
        echo "  $ sudo chown root:root $binarydir/wireleap_tun"
        echo "  $ sudo chmod u+s $binarydir/wireleap_tun"
    fi

    return 0
}

# wrap in function for some protection against partial file if "curl | sh"
main "$@"
