#!/bin/sh
set -e

fatal() { echo "fatal: $*" 1>&2; exit 1; }

usage() {
cat<<EOF
Syntax: $(basename "$0") binary
Generate CLI.md content from wireleap binary in PATH

Argument:
    binary      name of binary (eg. wireleap, wireleap-relay, etc..)

EOF
exit 1
}

_toc_link() {
    title="${1}"
    link=$(echo "${title}" | tr '[A-Z] ' '[a-z]-' | tr -d ':?,.()')
    echo "- [${title}](#${link})"
}

_help_entry() {
    printf "## $binary ${2}\n\n"
    printf '```\n'
    echo "$ $binary $@"
    printf "$($binary $@ 2>&1)\n"
    printf '```\n\n'
}

_cmds() {
    $binary help "${1}" 2>&1 | awk '/^  [a-z\-]/ {print $1}'
}

main() {
    [ -n "$1" ] || usage
    binary="$1"
    command -v ${binary} >/dev/null || fatal "${binary} not found"

    component=$(echo ${binary} | cut -d- -f2)
    [ "${component}" = "wireleap" ] && component="client"
    printf "# Wireleap ${component} command line reference\n\n"

    printf "## Table of contents\n\n"
    _toc_link "${binary}"
    for c in $(_cmds | grep -v help); do _toc_link "${binary} ${c}"; done

    echo
    _help_entry "help"
    for c in $(_cmds | grep -v help); do _help_entry "help" "${c}"; done

    echo -n "Generated CLI reference using ${binary} v" 1>&2
    echo "$(${binary} version)" 1>&2
}

main "$@"
