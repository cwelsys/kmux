#!/bin/zsh
# Unified session manager - attach/delete/rename sessions + zmx picker

SESSIONS_DIR="$HOME/.config/kitty/sessions"
mkdir -p "$SESSIONS_DIR"

# Function to extract zmx session names from a session file
get_zmx_sessions() {
    grep -o 'zmx attach [^ ]*' "$1" 2>/dev/null | sed 's/zmx attach //'
}

# Function to kill zmx sessions for a layout
kill_layout_sessions() {
    local session_file="$1"
    for zmx_name in $(get_zmx_sessions "$session_file"); do
        zmx kill "$zmx_name" 2>/dev/null
    done
}

# Function to show zmx session picker
pick_zmx_session() {
    local sessions=$(zmx list 2>/dev/null | while read -r line; do
        local name="" clients=""
        for field in ${(s:	:)line}; do
            case "$field" in
                session_name=*) name="${field#session_name=}" ;;
                clients=*) clients="${field#clients=}" ;;
            esac
        done
        [[ -n "$name" ]] && echo "$name ($clients clients)"
    done)

    [[ -z "$sessions" ]] && { echo "No zmx sessions"; sleep 1; return 1; }

    local selected=$(echo "$sessions" | fzf \
        --prompt="zmx > " \
        --reverse --border \
        --header="enter:attach | ctrl-k:kill | tab:layouts" \
        --expect="ctrl-k,tab")

    local key=$(echo "$selected" | head -1)
    local choice=$(echo "$selected" | tail -1 | sed 's/ (.*//')

    case "$key" in
        ctrl-k)
            [[ -n "$choice" ]] && zmx kill "$choice" 2>/dev/null
            pick_zmx_session
            ;;
        tab)
            return 0
            ;;
        *)
            [[ -n "$choice" ]] && exec zmx attach "$choice"
            ;;
    esac
}

mode="sessions"

while true; do
    if [[ "$mode" == "zmx" ]]; then
        pick_zmx_session
        mode="sessions"
        continue
    fi

    files=("$SESSIONS_DIR"/*.kitty-session(N))
    if (( ${#files} == 0 )); then
        # No saved sessions, go straight to zmx picker
        pick_zmx_session || exit 0
        continue
    fi

    selected=$(printf '%s\n' "${files[@]:t:r}" | fzf \
        --prompt="Session > " \
        --reverse --border \
        --header="enter:attach | ctrl-d:delete | ctrl-r:rename | tab:zmx" \
        --preview="cat $SESSIONS_DIR/{}.kitty-session" \
        --expect="ctrl-d,ctrl-r,tab")

    key=$(echo "$selected" | head -1)
    choice=$(echo "$selected" | tail -1)

    [[ -z "$choice" && "$key" != "tab" ]] && exit 0

    session_file="$SESSIONS_DIR/$choice.kitty-session"

    case "$key" in
        tab)
            mode="zmx"
            ;;
        ctrl-d)
            # Delete: kill zmx sessions and remove file
            kill_layout_sessions "$session_file"
            rm "$session_file" 2>/dev/null
            ;;
        ctrl-r)
            # Rename
            printf "New name: "
            read -r newname
            if [[ -n "$newname" ]]; then
                mv "$session_file" "$SESSIONS_DIR/$newname.kitty-session" 2>/dev/null
            fi
            ;;
        *)
            # Attach
            [[ ! -f "$session_file" ]] && exit 1

            original_tab_id=$(kitty @ ls | jq -r '.[0].tabs[] | select(.is_focused) | .id')
            current_cwd="$HOME"
            next_split="vsplit"
            in_first_tab=true
            first_window_in_tab=true
            tabs_created=0

            while IFS= read -r line || [[ -n "$line" ]]; do
                [[ -z "$line" ]] && continue
                case "$line" in
                    new_tab*)
                        if ! $in_first_tab; then first_window_in_tab=true; fi
                        in_first_tab=false
                        next_split="vsplit"
                        ;;
                    "cd "*) current_cwd="${line#cd }" ;;
                    "split "*) next_split="${line#split }" ;;
                    "layout "*) ;;
                    "launch "*)
                        cmd="${line#launch }"
                        if [[ "$cmd" == *"zmx attach"* ]]; then
                            zmx_session="${cmd##*zmx attach }"
                        else
                            zmx_session=""
                        fi

                        if $first_window_in_tab; then
                            if [[ -n "$zmx_session" ]]; then
                                kitty @ launch --type=tab --cwd="$current_cwd" --env KMUX_MODE=1 --env "KMUX_SESSION=$zmx_session" --env "KMUX_LAYOUT=$choice" >/dev/null
                            else
                                kitty @ launch --type=tab --cwd="$current_cwd" --env "KMUX_LAYOUT=$choice" >/dev/null
                            fi
                            ((tabs_created++))
                            first_window_in_tab=false
                        else
                            if [[ -n "$zmx_session" ]]; then
                                kitty @ launch --location="$next_split" --cwd="$current_cwd" --env KMUX_MODE=1 --env "KMUX_SESSION=$zmx_session" --env "KMUX_LAYOUT=$choice" >/dev/null
                            else
                                kitty @ launch --location="$next_split" --cwd="$current_cwd" --env "KMUX_LAYOUT=$choice" >/dev/null
                            fi
                        fi
                        next_split="vsplit"
                        ;;
                esac
            done < "$session_file"

            if (( tabs_created > 0 )) && [[ -n "$original_tab_id" ]]; then
                kitty @ close-tab --match "id:$original_tab_id" 2>/dev/null
            fi
            exit 0
            ;;
    esac
done
