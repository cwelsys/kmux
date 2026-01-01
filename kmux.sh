#!/bin/zsh
# kmux - kitty + zmx session manager

SESSIONS_DIR="${HOME}/.config/kitty/sessions"

kmux_list() {
    local output=$(zmx list 2>/dev/null)
    [[ -z "$output" ]] && { echo "No zmx sessions"; return; }

    echo "$output" | while read -r line; do
        [[ -z "$line" ]] && continue
        local name="" clients=""
        for field in ${(s:	:)line}; do
            case "$field" in
                session_name=*) name="${field#session_name=}" ;;
                clients=*) clients="${field#clients=}" ;;
            esac
        done
        [[ -z "$name" ]] && continue
        if [[ "$clients" == "0" ]]; then
            echo "○ $name"
        elif [[ "$clients" =~ ^[0-9]+$ ]]; then
            echo "● $name ($clients)"
        else
            echo "○ $name"
        fi
    done
}

kmux_clean() {
    local -a to_kill=()
    local output=$(zmx list 2>/dev/null)
    [[ -z "$output" ]] && { echo "No sessions"; return; }

    # Collect sessions with 0 clients
    while read -r line; do
        [[ -z "$line" ]] && continue
        local name="" clients=""
        for field in ${(s:	:)line}; do
            case "$field" in
                session_name=*) name="${field#session_name=}" ;;
                clients=*) clients="${field#clients=}" ;;
            esac
        done
        [[ "$clients" == "0" && -n "$name" ]] && to_kill+=("$name")
    done <<< "$output"

    (( ${#to_kill} == 0 )) && { echo "No orphaned sessions"; return; }

    # Kill them
    for name in "${to_kill[@]}"; do
        zmx kill "$name" 2>/dev/null
        echo "Killed: $name"
    done

    echo "Cleaned ${#to_kill} session(s)"
}

kmux_attach() {
    local sessions=$(zmx list 2>/dev/null | sed 's/session_name=\([^	]*\).*/\1/')
    [[ -z "$sessions" ]] && { echo "No sessions"; return 1; }
    local selected=$(echo "$sessions" | fzf --prompt="zmx > " --reverse --border)
    [[ -n "$selected" ]] && exec zmx attach "$selected"
}

kmux_sessions() {
    mkdir -p "$SESSIONS_DIR"
    local files=("$SESSIONS_DIR"/*.kitty-session(N))
    (( ${#files} == 0 )) && { echo "No saved sessions"; return 1; }
    local selected=$(printf '%s\n' "${files[@]:t:r}" | fzf --prompt="session > " --reverse --border \
        --preview "cat $SESSIONS_DIR/{}.kitty-session")
    [[ -n "$selected" ]] && echo "$selected"
}

kmux_save() {
    mkdir -p "$SESSIONS_DIR"
    printf "Name: "; read -r name
    [[ -z "$name" ]] && return

    session_file="$SESSIONS_DIR/$name.kitty-session"

    # Get current state and convert to kitty session format
    kitty @ ls | jq -r '
    .[0].tabs[] |
    [.windows[] | select(.is_self == false or .is_self == null)] as $wins |
    ($wins | first) as $main_win |
    ($main_win.cwd | split("/") | last) as $tab_title |
    (if .layout_state.pairs.horizontal == true then "vsplit" else "hsplit" end) as $split |
    "new_tab \($tab_title)
layout \(.layout)
cd \($main_win.cwd // env.HOME)
" + (
      [range($wins | length) | . as $i |
        $wins[$i] as $win |
        (if $i == 0 then "" else "split \($split)\n" end) +
        "launch " + (
          if $win.foreground_processes[0].cmdline[0] == "zmx" then
            $win.foreground_processes[0].cmdline | join(" ")
          elif ($win.foreground_processes[0].cmdline[0] | test("^-?zsh$")) then
            ""
          else
            $win.foreground_processes[0].cmdline | join(" ")
          end
        )
      ] | join("\n")
    )
    ' > "$session_file"

    # Remove empty launch lines
    sed -i '' '/^launch $/d' "$session_file"

    echo "Saved: $name"
}

kmux_start() {
    # Start kmux mode - set KMUX_MODE so new splits also auto-attach
    export KMUX_MODE=1
    exec zmx attach "kmux-${KITTY_WINDOW_ID}"
}

kmux_delete() {
    mkdir -p "$SESSIONS_DIR"
    local files=("$SESSIONS_DIR"/*.kitty-session(N))
    (( ${#files} == 0 )) && { echo "No saved sessions"; return 1; }

    local selected=$(printf '%s\n' "${files[@]:t:r}" | fzf --prompt="Delete session > " --reverse --border \
        --preview "cat $SESSIONS_DIR/{}.kitty-session" \
        --header "Press enter to delete, ctrl-c to cancel")

    [[ -z "$selected" ]] && return 0

    rm "$SESSIONS_DIR/$selected.kitty-session" && echo "Deleted: $selected"
}

kmux_rename() {
    mkdir -p "$SESSIONS_DIR"
    local files=("$SESSIONS_DIR"/*.kitty-session(N))
    (( ${#files} == 0 )) && { echo "No saved sessions"; return 1; }

    local selected=$(printf '%s\n' "${files[@]:t:r}" | fzf --prompt="Rename session > " --reverse --border \
        --preview "cat $SESSIONS_DIR/{}.kitty-session")

    [[ -z "$selected" ]] && return 0

    printf "New name: "; read -r newname
    [[ -z "$newname" ]] && return 0

    mv "$SESSIONS_DIR/$selected.kitty-session" "$SESSIONS_DIR/$newname.kitty-session" && echo "Renamed: $selected -> $newname"
}

case "$1" in
    list|ls) kmux_list ;;
    clean) kmux_clean ;;
    kill) shift; zmx kill "$@" ;;
    attach|a) kmux_attach ;;
    sessions|s) kmux_sessions ;;
    save) kmux_save ;;
    delete|rm) kmux_delete ;;
    rename|mv) kmux_rename ;;
    help|-h|--help)
        cat <<'EOF'
kmux - kitty + zmx session manager

Usage: kmux [command]

  (no args)    Start persistent session (zmx mode)
  list, ls     List zmx sessions
  clean        Kill detached sessions
  kill <name>  Kill specific session
  attach, a    Pick and attach to zmx session
  sessions, s  Pick saved kitty layout
  save         Save current kitty layout
  delete, rm   Delete a saved layout
  rename, mv   Rename a saved layout
EOF
        ;;
    "") kmux_start ;;
    *) echo "Unknown command: $1"; exit 1 ;;
esac
