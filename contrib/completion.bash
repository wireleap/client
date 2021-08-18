# bashrc: source path/to/completion.bash
# depends: jq
# config circuit.whitelist gotchas:
#   - needs to be manually prefixed with " pre TAB
#   - post-completion needs to be wrapped '[ ... ]' and comma-delimited

__wireleap_cmds() {
    wireleap help $1 2>&1 | awk '/^  [a-z\-]/ {print $1}'
}

__wireleap_scripts() {
    command -v jq >/dev/null || return 1
    local wlhome="$(wireleap info | jq -r '.wireleap_home')"
    [ -d "${wlhome}/scripts" ] || return 1
    find ${wlhome}/scripts -executable -type f -printf "%f\n" | uniq
}

__wireleap_relays() {
    command -v jq >/dev/null || return 1
    local wlhome="$(wireleap info | jq -r '.wireleap_home')"
    [ -f "${wlhome}/relays.json" ] || return 1
    jq '.[].address' < ${wlhome}/relays.json
}

__wireleap_comp() {
    case "${#COMP_WORDS[@]}" in
        2)
            local words="$(__wireleap_cmds)";
            ;;
        3)
            case "${COMP_WORDS[1]}" in
                exec) local words="$(__wireleap_scripts)";;
                *)    local words="$(__wireleap_cmds ${COMP_WORDS[1]})";;
            esac
            ;;
        *)
            case "${COMP_WORDS[2]}" in
                circuit.whitelist) local words="$(__wireleap_relays)";;
                *)                 return 1;;
            esac
            ;;
    esac
    COMPREPLY=($(compgen -W "$words" -- "${COMP_WORDS[COMP_CWORD]}"));
}

complete -o default -F __wireleap_comp wireleap
