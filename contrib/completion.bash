# bashrc: source path/to/completion.bash

__wireleap_cmds() {
    wireleap help $1 2>&1 | awk '/^  [a-z\-]/ {print $1}'
}

__wireleap_comp() {
    case "${#COMP_WORDS[@]}" in
        2)
            local words="$(__wireleap_cmds)";
            ;;
        3)

            local words="$(__wireleap_cmds ${COMP_WORDS[1]})";
            ;;
        *)
            return 1
            ;;
    esac
    COMPREPLY=($(compgen -W "$words" -- "${COMP_WORDS[COMP_CWORD]}"));
}

complete -o default -F __wireleap_comp wireleap
