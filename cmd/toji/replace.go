package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mokkori-Tom/toji/internal/cerr"
	"github.com/Mokkori-Tom/toji/internal/concept"
	"github.com/Mokkori-Tom/toji/internal/pageappend"
	"github.com/Mokkori-Tom/toji/internal/pagereplace"
	"github.com/Mokkori-Tom/toji/internal/show"
	"github.com/Mokkori-Tom/toji/internal/textdiff"
)

const replaceUsage = "usage: toji replace [bundle] <concept-id | file.md> --old <text>|- (--new <text>|- | --delete) [--section <heading>] [--all] [--dry-run] [--diff]"

// runReplace changes one exact piece of an existing page:
// `toji replace [bundle] <concept-id | file.md> --old <text>|- (--new <text>|- | --delete) [--section <heading>] [--all] [--dry-run] [--diff]`.
func runReplace(args []string) {
	var old, repl, section string
	hasOld, hasNew, del, all, dryRun, showDiff := false, false, false, false, false, false
	var pos []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--old", "--new", "--section":
			if i+1 >= len(args) {
				exitErr(cerr.Usage("%s requires a value", args[i]))
			}
			switch args[i] {
			case "--old":
				old, hasOld = args[i+1], true
			case "--new":
				repl, hasNew = args[i+1], true
			default:
				section = args[i+1]
			}
			i++
		case "--delete":
			del = true
		case "--all":
			all = true
		case "--dry-run":
			dryRun = true
		case "--diff":
			showDiff = true
		default:
			if strings.HasPrefix(args[i], "-") {
				exitErr(cerr.Usage("unknown replace flag: %s", args[i]))
			}
			pos = append(pos, args[i])
		}
	}
	if len(pos) == 1 {
		pos = []string{defaultBundle(), pos[0]}
	}
	switch {
	case len(pos) != 2 || !hasOld || hasNew == del:
		exitErr(cerr.Usage(replaceUsage))
	case old == "-" && repl == "-":
		exitErr(cerr.Usage("only one of --old and --new can read stdin"))
	}
	if old == "-" || (hasNew && repl == "-") {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			exitErr(cerr.IO(err, "read stdin"))
		}
		// A heredoc ends with a line break the user did not mean as text.
		text := strings.TrimSuffix(string(raw), "\n")
		if old == "-" {
			old = text
		} else {
			repl = text
		}
	}

	var file, id string
	if isMarkdownFile(pos[1]) {
		file, _ = filepath.Abs(pos[1])
		id = strings.TrimSuffix(filepath.Base(file), ".md")
	} else {
		b := mustBundle("replace", pos[:1])
		c, err := show.Show(b, strings.TrimSuffix(pos[1], ".md"))
		if err != nil {
			exitErr(cerr.Validation("%s", err))
		}
		file, id = c.Path, c.ID
	}

	raw, err := os.ReadFile(file) //nolint:gosec // the page the user named
	if err != nil {
		exitErr(cerr.IO(err, "read %s", file))
	}
	out, lines, err := pagereplace.Replace(string(raw), old, repl, section, all)
	if err != nil {
		var nf *pagereplace.NotFoundError
		var amb *pagereplace.AmbiguousError
		var snf *pageappend.NotFoundError
		var samb *pageappend.AmbiguousError
		if errors.As(err, &nf) || errors.As(err, &amb) || errors.As(err, &snf) || errors.As(err, &samb) {
			exitErr(cerr.Validation("%s: %s", id, err))
		}
		exitErr(cerr.Usage("%s", err))
	}
	// A page whose frontmatter parsed before must still parse after: a slip
	// inside the YAML would otherwise drop the page from the bundle.
	if _, err := concept.ParseBytes(raw, id, file); err == nil {
		if _, err := concept.ParseBytes([]byte(out), id, file); err != nil {
			exitErr(cerr.Validation("%s: the change would break the frontmatter, nothing written: %s", id, err))
		}
	}
	if !dryRun {
		if err := os.WriteFile(file, []byte(out), 0644); err != nil { //nolint:gosec // the page the user named
			exitErr(cerr.IO(err, "write %s", file))
		}
	}
	diff := ""
	if showDiff {
		diff = textdiff.UnifiedDiff(string(raw), out, filepath.Base(file))
	}
	if humanOut {
		verb := "replaced"
		if dryRun {
			verb = "would replace"
		}
		fmt.Fprintf(os.Stderr, "%s %d occurrence(s) at line(s) %v: %s\n", verb, len(lines), lines, file)
		fmt.Print(diff)
		return
	}
	res := map[string]any{
		"command": "replace",
		"id":      id,
		"path":    file,
		"section": section,
		"count":   len(lines),
		"lines":   lines,
		"deleted": del,
		"dry_run": dryRun,
	}
	if showDiff {
		res["diff"] = diff
	}
	outputJSON(res)
}
