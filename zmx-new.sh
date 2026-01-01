#!/bin/zsh
# Create a new zmx session with optional name prompt
# Usage: zmx-new.sh [--prompt]

# Source zsh profile to get PATH
[[ -f ~/.zshrc ]] && source ~/.zshrc 2>/dev/null

ZMX=$(command -v zmx)

if [[ "$1" == "--prompt" ]]; then
    # Prompt for session name
    printf "Session name (enter for auto): "
    read -r name
fi

if [[ -z "$name" ]]; then
    # Auto-generate name from directory + short random
    dir=$(basename "$PWD")
    rand=$(head -c 4 /dev/urandom | xxd -p | head -c 4)
    name="${dir}-${rand}"
fi

exec $ZMX attach "$name"
