#!/bin/zsh
# Attach: pick a saved session and recreate it in current OS window

SESSIONS_DIR="$HOME/.config/kitty/sessions"
mkdir -p "$SESSIONS_DIR"

# List session files
files=("$SESSIONS_DIR"/*.kitty-session(N))
(( ${#files} == 0 )) && { echo "No saved sessions"; sleep 1; exit 1; }

# Pick with fzf
selected=$(printf '%s\n' "${files[@]:t:r}" | fzf --prompt="Session > " --reverse --border \
    --preview "cat $SESSIONS_DIR/{}.kitty-session")

[[ -z "$selected" ]] && exit 0

session_file="$SESSIONS_DIR/$selected.kitty-session"
[[ ! -f "$session_file" ]] && exit 1

# Track original state
original_tab_id=$(kitty @ ls | jq -r '.[0].tabs[] | select(.is_focused) | .id')
original_tab_count=$(kitty @ ls | jq '.[0].tabs | length')

# Parse session file and recreate
current_cwd="$HOME"
next_split="vsplit"
in_first_tab=true
first_window_in_tab=true
tabs_created=0

while IFS= read -r line || [[ -n "$line" ]]; do
    # Skip empty lines
    [[ -z "$line" ]] && continue

    case "$line" in
        new_tab*)
            if ! $in_first_tab; then
                first_window_in_tab=true
            fi
            in_first_tab=false
            next_split="vsplit"
            ;;
        "cd "*)
            current_cwd="${line#cd }"
            ;;
        "split "*)
            next_split="${line#split }"
            ;;
        "layout "*)
            ;;
        "launch "*)
            cmd="${line#launch }"

            # Extract zmx session name if this is a zmx command
            if [[ "$cmd" == *"zmx attach"* ]]; then
                zmx_session="${cmd##*zmx attach }"
            else
                zmx_session=""
            fi

            if $first_window_in_tab; then
                if [[ -n "$zmx_session" ]]; then
                    kitty @ launch --type=tab --cwd="$current_cwd" --env KMUX_MODE=1 --env "KMUX_SESSION=$zmx_session" --env "KMUX_LAYOUT=$selected" >/dev/null
                else
                    kitty @ launch --type=tab --cwd="$current_cwd" --env "KMUX_LAYOUT=$selected" >/dev/null
                fi
                ((tabs_created++))
                first_window_in_tab=false
            else
                if [[ -n "$zmx_session" ]]; then
                    kitty @ launch --location="$next_split" --cwd="$current_cwd" --env KMUX_MODE=1 --env "KMUX_SESSION=$zmx_session" --env "KMUX_LAYOUT=$selected" >/dev/null
                else
                    kitty @ launch --location="$next_split" --cwd="$current_cwd" --env "KMUX_LAYOUT=$selected" >/dev/null
                fi
            fi
            # Reset to default for next window
            next_split="vsplit"
            ;;
    esac
done < "$session_file"

# Close original tab if we created new ones
if (( tabs_created > 0 )) && [[ -n "$original_tab_id" ]]; then
    kitty @ close-tab --match "id:$original_tab_id" 2>/dev/null
fi

