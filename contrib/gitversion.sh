#!/bin/sh -e

SCRIPT_NAME="$(basename "$0")"
fatal() { echo "FATAL [$SCRIPT_NAME]: $*" 1>&2; exit 1; }

command -v git >/dev/null || fatal "git not installed"

if git describe > /dev/null 2>&1; then
    git describe | sed 's/^v//'
else
    echo "0.0.0-$(git describe --always)"
fi
