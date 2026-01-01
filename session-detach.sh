#!/bin/zsh
# Detach: save layout to kitty session format, close tabs, leave fresh shell

SESSIONS_DIR="$HOME/.config/kitty/sessions"
mkdir -p "$SESSIONS_DIR"

# Get session name (auto-use KMUX_LAYOUT if set)
if [[ -n "$KMUX_LAYOUT" ]]; then
    name="$KMUX_LAYOUT"
else
    printf "Session name: "
    read -r name
    [[ -z "$name" ]] && exit 0
fi

session_file="$SESSIONS_DIR/$name.kitty-session"

# Get current state
state=$(kitty @ ls)

# Convert to kitty session format
# Use layout_state.pairs.horizontal to determine split direction
# horizontal: true = side by side (vsplit), horizontal: false = stacked (hsplit)
echo "$state" | jq -r '
.[0].tabs[] |
[.windows[] | select(.is_self == false or .is_self == null)] as $wins |
($wins | first) as $main_win |
($main_win.cwd | split("/") | last) as $tab_title |
# Get split direction from layout_state
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

# Count tabs
num_tabs=$(echo "$state" | jq '.[0].tabs | length')
((num_tabs < 1)) && num_tabs=0

# Launch fresh tab
kitty @ launch --type=tab --cwd="$HOME" >/dev/null

# Close all old tabs (including the one with overlay)
for ((i=0; i<num_tabs; i++)); do
    kitty @ close-tab --match recent:1 2>/dev/null
done
