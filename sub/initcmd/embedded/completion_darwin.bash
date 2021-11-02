# bashrc: source path/to/completion_darwin.bash
# zshrc: autoload compinit && compinit && autoload bashcompinit && bashcompinit && source path/to/completion_darwin.bash

__wireleap_cmds() {
    wireleap help $1 2>&1 | awk '/^  [a-z\-]/ {print $1}'
}

__wireleap_home() {
    wireleap info | sed -n 's/^.*\"wireleap_home\": \"\(.*\)\",$/\1/p'
}

__wireleap_scripts() {
    local wlhome="$(__wireleap_home)"
    [ -d "${wlhome}/scripts" ] || return 1
    find ${wlhome}/scripts -perm +111 -type f -exec basename '{}' ';' | uniq
}

__wireleap_relays() {
    local wlhome="$(__wireleap_home)"
    [ -f "${wlhome}/relays.json" ] || return 1
    sed -n 's/^.*\"address\": \"\(.*\)\",$/\1/p' < ${wlhome}/relays.json
}

__wireleap_comp() {
    local words
    case "${#COMP_WORDS[@]}" in
        1)
            words="$(__wireleap_cmds)";
            ;;
        *)
            case "${COMP_WORDS[1]}" in
                exec) words="$(__wireleap_scripts)";;
                config)
                    case "${COMP_WORDS[2]}" in
                        circuit.whitelist)
                            local cur="${COMP_WORDS[COMP_CWORD]}"
                            words="$(__wireleap_relays)"
                            COMPREPLY=($(compgen -W "$words" -- "$cur"))
                            return 0
                            ;;
                        *) words="$(__wireleap_cmds ${COMP_WORDS[1]})";;
                    esac
                    ;;
                *) words="$(__wireleap_cmds ${COMP_WORDS[1]})";;
            esac
            ;;
    esac
    COMPREPLY=($(compgen -W "$words" -- "${COMP_WORDS[COMP_CWORD]}"))
}

complete -o default -F __wireleap_comp wireleap
