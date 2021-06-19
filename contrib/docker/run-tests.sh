#!/bin/sh
set -e

SCRIPT_NAME="$(basename "$0")"

fatal() { echo "FATAL [$SCRIPT_NAME]: $*" 1>&2; exit 1; }

command -v docker >/dev/null || fatal "docker not installed"

SRCDIR="$(dirname "$(dirname "$(dirname "$(realpath "$0")")")")"

. "$SRCDIR/contrib/docker/goversion.sh"
[ -n "$GO_VERSION" ] || fatal "go version is not defined"

if [ -n "$DEPS_CACHE" ]; then
    DEPS_CACHE="$(realpath "$DEPS_CACHE")"
    [ -d "$DEPS_CACHE" ] || fatal "does not exist: $DEPS_CACHE"
    DEPS_CACHE_OPTS="-v $DEPS_CACHE:/go/deps -e GOPATH=/go/deps:/go"
fi

docker run --rm \
    -v "$SRCDIR:/go/src/wireleap" \
    -w /go/src/wireleap \
    -e GITHUB_TOKEN \
    $DEPS_CACHE_OPTS \
    "golang:$GO_VERSION" /go/src/wireleap/contrib/run-tests.sh "$@"

