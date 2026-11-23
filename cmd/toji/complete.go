package main

// Shell completion. `toji completion zsh|bash` prints a script; the script
// asks the hidden `toji __complete <kind> [bundle]` for candidates, one per
// line, so page IDs, tags and types always come from the live bundle.

import (
	"fmt"
	"os"
	"sort"

	"github.com/Mokkori-Tom/toji/internal/bundle"
	"github.com/Mokkori-Tom/toji/internal/cerr"
)

func runCompletion(args []string) {
	if len(args) != 1 {
		exitErr(cerr.Usage("usage: toji completion zsh|bash"))
	}
	switch args[0] {
	case "zsh":
		fmt.Print(zshCompletion)
	case "bash":
		fmt.Print(bashCompletion)
	default:
		exitErr(cerr.Usage("unsupported shell %q: use zsh or bash", args[0]))
	}
}

// runComplete prints candidates for the completion scripts. It never fails
// loudly: a missing bundle just yields no candidates.
func runComplete(args []string) {
	if len(args) == 0 {
		return
	}
	root := defaultBundle()
	if len(args) > 1 && isDir(args[1]) {
		root = args[1]
	}
	if args[0] == "templates" {
		for _, n := range templateNames(root) {
			fmt.Fprintln(os.Stdout, n)
		}
		return
	}
	b, err := bundle.Load(root)
	if err != nil {
		return
	}
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, c := range b.Concepts {
		switch args[0] {
		case "ids":
			add(c.ID)
		case "tags":
			for _, t := range c.Frontmatter.Tags {
				add(t)
			}
		case "types":
			add(c.Frontmatter.Type)
		}
	}
	sort.Strings(out)
	for _, s := range out {
		fmt.Fprintln(os.Stdout, s)
	}
}

const zshCompletion = `#compdef toji
# toji completion for zsh. Load with:
#   toji completion zsh >/dev/null 2>&1 && source <(toji completion zsh)

_okf_lines() {
  local -a lines
  lines=("${(@f)$(toji __complete "$@" 2>/dev/null)}")
  print -rl -- "${lines[@]}"
}

_okf() {
  local -a cmds
  cmds=(
    'search:search pages by text, --tag, --type'
    'show:show a page (rendered on a terminal)'
    'edit:open a page in $EDITOR'
    'toc:write a table of contents into a page'
    'append:add markdown to the end of a page or a section'
    'replace:change one exact piece of a page'
    'new:create a page and open it in $EDITOR'
    'list:list pages'
    'backlinks:list pages that link to a page'
    'validate:check the bundle against the OKF spec'
    'lint:recommended fields and style (warnings only)'
    'index:generate index.md files'
    'graph:cross-link graph statistics'
    'init:create a new bundle'
    'schema:machine-readable CLI description'
    'completion:print a shell completion script'
    'version:print version'
  )
  if (( CURRENT == 2 )); then
    _describe -t commands 'toji command' cmds
    return
  fi

  local cmd=$words[2] cur=$words[CURRENT] prev=$words[CURRENT-1]
  local bundle=""
  [[ -n $words[3] && -d ${~words[3]} && CURRENT -gt 3 ]] && bundle=${~words[3]}
  local -a common=(--json --human)

  case $prev in
    --tag)  compadd -- ${(f)"$(_okf_lines tags $bundle)"}; return ;;
    --type) compadd -- ${(f)"$(_okf_lines types $bundle)"}; return ;;
    --template) compadd -- ${(f)"$(_okf_lines templates $bundle)"}; _files -g '*.md'; return ;;
    --title|--description|--text|--body|--section|--old|--new) return ;;
    --format) compadd -- json sarif; return ;;
    --id) compadd -- ${(f)"$(_okf_lines ids $bundle)"}; return ;;
    --dir) _files -/; return ;;
  esac

  if [[ $cur == -* ]]; then
    case $cmd in
      show)     compadd -- --raw --no-pager $common ;;
      search)   compadd -- --tag --type --text --meta --limit $common ;;
      list|graph) compadd -- --limit $common ;;
      new)      compadd -- --title --type --tag --description --status --body --template --no-edit $common ;;
      toc)      compadd -- --depth --remove --dry-run $common ;;
      append)   compadd -- --body --section --dry-run $common ;;
      replace)  compadd -- --old --new --delete --section --all --dry-run --diff $common ;;
      index)    compadd -- --dir --diff --dry-run --fill-descriptions $common ;;
      validate) compadd -- --format --exit-zero --errors-only --id --strict $common ;;
      lint)     compadd -- --format --id --strict $common ;;
      *)        compadd -- $common ;;
    esac
    return
  fi

  case $cmd in
    show|edit|toc|append|replace|backlinks)
      local -a ids; ids=(${(f)"$(_okf_lines ids $bundle)"})
      _wanted pages expl 'page' compadd -a ids
      [[ $cmd != backlinks ]] && _files -g '*.md'
      ;;
    new)
      # Suggest existing folders inside the bundle as id prefixes.
      local -a ids dirs; ids=(${(f)"$(_okf_lines ids $bundle)"})
      dirs=(${(u)${(M)ids:#*/*}%/*})
      _wanted folders expl 'folder' compadd -S / -q -a dirs
      ;;
    completion) compadd -- zsh bash ;;
    schema) compadd -- ${cmds%%:*} ;;
    list|validate|lint|index|graph|init|search) _files -/ ;;
  esac
}

compdef _okf toji
`

const bashCompletion = `# toji completion for bash. Load with:
#   toji completion bash >/dev/null 2>&1 && source <(toji completion bash)

_okf() {
  local cur prev cmd bundle=""
  cur=${COMP_WORDS[COMP_CWORD]}
  prev=${COMP_WORDS[COMP_CWORD-1]}
  cmd=${COMP_WORDS[1]}
  local IFS=$'\n'

  if (( COMP_CWORD == 1 )); then
    COMPREPLY=($(compgen -W $'search\nshow\nedit\ntoc\nappend\nreplace\nnew\nlist\nbacklinks\nvalidate\nlint\nindex\ngraph\ninit\nschema\ncompletion\nversion' -- "$cur"))
    return
  fi
  if (( COMP_CWORD > 2 )) && [[ -d ${COMP_WORDS[2]} ]]; then
    bundle=${COMP_WORDS[2]}
  fi

  case $prev in
    --tag)  COMPREPLY=($(compgen -W "$(toji __complete tags $bundle 2>/dev/null)" -- "$cur")); compopt -o filenames; return ;;
    --type) COMPREPLY=($(compgen -W "$(toji __complete types $bundle 2>/dev/null)" -- "$cur")); compopt -o filenames; return ;;
    --template) COMPREPLY=($(compgen -W "$(toji __complete templates $bundle 2>/dev/null)" -- "$cur")); compopt -o filenames; return ;;
    --title|--description|--text|--body|--section|--old|--new) return ;;
    --format) COMPREPLY=($(compgen -W $'json\nsarif' -- "$cur")); return ;;
    --id) COMPREPLY=($(compgen -W "$(toji __complete ids $bundle 2>/dev/null)" -- "$cur")); compopt -o filenames; return ;;
    --dir) COMPREPLY=($(compgen -d -- "$cur")); compopt -o filenames; return ;;
  esac

  if [[ $cur == -* ]]; then
    local flags
    case $cmd in
      show)     flags=$'--raw\n--no-pager' ;;
      search)   flags=$'--tag\n--type\n--text\n--meta\n--limit' ;;
      list|graph) flags=$'--limit' ;;
      new)      flags=$'--title\n--type\n--tag\n--description\n--status\n--body\n--template\n--no-edit' ;;
      toc)      flags=$'--depth\n--remove\n--dry-run' ;;
      append)   flags=$'--body\n--section\n--dry-run' ;;
      replace)  flags=$'--old\n--new\n--delete\n--section\n--all\n--dry-run\n--diff' ;;
      index)    flags=$'--dir\n--diff\n--dry-run\n--fill-descriptions' ;;
      validate) flags=$'--format\n--exit-zero\n--errors-only\n--id\n--strict' ;;
      lint)     flags=$'--format\n--id\n--strict' ;;
    esac
    COMPREPLY=($(compgen -W "$flags"$'\n--json\n--human' -- "$cur"))
    return
  fi

  case $cmd in
    show|edit|toc|append|replace|backlinks)
      COMPREPLY=($(compgen -W "$(toji __complete ids $bundle 2>/dev/null)" -- "$cur"))
      [[ $cmd != backlinks ]] && COMPREPLY+=($(compgen -f -X '!*.md' -- "$cur"))
      compopt -o filenames
      ;;
    completion) COMPREPLY=($(compgen -W $'zsh\nbash' -- "$cur")) ;;
    list|validate|lint|index|graph|init|search) COMPREPLY=($(compgen -d -- "$cur")); compopt -o filenames ;;
  esac
}

complete -F _okf toji
`
