#!/bin/sh
set -e

SCRIPT_NAME="$(basename "$0")"

fatal() { echo "FATAL [$SCRIPT_NAME]: $*" 1>&2; exit 1; }
info() { echo "INFO [$SCRIPT_NAME]: $*"; }

command -v go >/dev/null || fatal "go not installed"

SRCDIR="$(dirname "$(dirname "$(realpath "$0")")")"
GITVERSION="$($SRCDIR/contrib/gitversion.sh)"

NPROC=
if command -v nproc >/dev/null; then
    NPROC="$( nproc )"
elif command -v grep >/dev/null; then
    NPROC="$( grep -c processor /proc/cpuinfo )"
fi

if [ "$NPROC" -lt 2 ]; then
    NPROC=2
fi

info "running at most $NPROC tests in parallel"

GOOS=${GOOS:-$(go env GOOS)}

if [ "$GOOS" = 'linux' ]; then
    info "compiling wireleap_intercept (needed for wireleap on linux)"
    make -C wireleap_intercept
    cp wireleap_intercept/wireleap_intercept.so "$SRCDIR/sub/initcmd/embedded"

    info "building wireleap_tun"
    cd wireleap_tun
    go get -v -d ./...
    CGO_ENABLED=0 go build
    cd -
    mv wireleap_tun/wireleap_tun "$SRCDIR/sub/initcmd/embedded"
fi

VERSIONS=
for c in common/api common/cli client; do
    VERSIONS="$VERSIONS -X github.com/wireleap/$c/version.GITREV=$GITVERSION"
done

cp "$SRCDIR/LICENSE" "$SRCDIR/sub/initcmd/embedded/"

info "testing ..."
go test \
    -parallel "$NPROC" \
    -ldflags "
        $VERSIONS
        -X github.com/wireleap/common/wlnet.PROTO_VERSION_STRING=$GITVERSION \
        -X github.com/wireleap/common/api/apiversion.VERSION_STRING=$GITVERSION
    " \
    "$@" ./...
