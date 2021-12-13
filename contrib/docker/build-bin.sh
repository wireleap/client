#!/bin/sh
set -e

SCRIPT_NAME="$(basename "$0")"

fatal() { echo "FATAL [$SCRIPT_NAME]: $*" 1>&2; exit 1; }
info() { echo "INFO [$SCRIPT_NAME]: $*"; }

usage() {
cat<<EOF
Syntax: $SCRIPT_NAME /path/to/outdir
Helper script to compile Wireleap components (inside docker)

Environment::

    TARGET_OS       Optional (linux, darwin)
    DEPS_CACHE      Optional (path/to/depsdir)
    BUILD_TAGS      Optional (arbitrary tags)

EOF
exit 1
}

[ -n "$1" ] || usage

command -v docker >/dev/null || fatal "docker not installed"

SRCDIR="$(dirname "$(dirname "$(dirname "$(realpath "$0")")")")"

. "$SRCDIR/contrib/docker/goversion.sh"
[ -n "$GO_VERSION" ] || fatal "go version is not defined"

OUTDIR="$(realpath "$1")"
[ -d "$OUTDIR" ] || mkdir -p "$OUTDIR"

case "$TARGET_OS" in
    "")     GOOS=""; UNAME="";;
    linux)  GOOS="linux"; UNAME="Linux";;
    darwin) GOOS="darwin"; UNAME="Darwin";;
    windows) GOOS="windows"; UNAME="Windows";;
    *)      fatal "TARGET_OS not supported: $TARGET_OS";;
esac

if [ -n "$DEPS_CACHE" ]; then
    DEPS_CACHE="$(realpath "$DEPS_CACHE")"
    [ -d "$DEPS_CACHE" ] || fatal "does not exist: $DEPS_CACHE"
    DEPS_CACHE_OPTS="-v $DEPS_CACHE:/go/deps -e GOPATH=/go/deps:/go"
fi

docker run --rm -i \
    -v "$OUTDIR:/tmp/build" \
    -v "$SRCDIR:/go/src/wireleap" \
    -w /go/src/wireleap \
    -e "GOOS=$GOOS" \
    -e "UNAME=$UNAME" \
    -e "BUILD_USER=$(id -u "$USER")" \
    -e "BUILD_GROUP=$(id -u "$USER")" \
    -e "BUILD_TAGS=$BUILD_TAGS" \
    $DEPS_CACHE_OPTS \
    $DOCKER_OPTS \
    "golang:$GO_VERSION" /go/src/wireleap/contrib/build-bin.sh /tmp/build

