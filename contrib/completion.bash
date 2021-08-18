# bashrc: source path/to/completion.bash
# depends: jq

__wireleap_cmds() {
    wireleap help $1 2>&1 | awk '/^  [a-z\-]/ {print $1}'
}

__wireleap_scripts() {
    command -v jq >/dev/null || return 1
    local wlhome="$(wireleap info | jq -r '.wireleap_home')"
    [ -d "${wlhome}/scripts" ] || return 1
    find ${wlhome}/scripts -executable -type f -printf "%f\n" | uniq
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
            return 1
            ;;
    esac
    COMPREPLY=($(compgen -W "$words" -- "${COMP_WORDS[COMP_CWORD]}"));
}

complete -o default -F __wireleap_comp wireleap
