#!/bin/zsh
# Save: save current layout without closing tabs

SESSIONS_DIR="$HOME/.config/kitty/sessions"
mkdir -p "$SESSIONS_DIR"

# Get session name
printf "Session name: "
read -r name
[[ -z "$name" ]] && exit 0

session_file="$SESSIONS_DIR/$name.kitty-session"

# Get current state
state=$(kitty @ ls)

# Convert to kitty session format
# horizontal: true = vsplit, horizontal: false = hsplit
echo "$state" | jq -r '
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

