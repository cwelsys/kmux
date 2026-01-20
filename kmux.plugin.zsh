if (( ! $+commands[kmux] )); then
  return
fi

if [[ ! -f "$ZSH_CACHE_DIR/completions/_kmux" ]]; then
  typeset -g -A _comps
  autoload -Uz _kmux
  _comps[kmux]=_kmux
fi

kmux completion zsh >| "$ZSH_CACHE_DIR/completions/_kmux" &|
