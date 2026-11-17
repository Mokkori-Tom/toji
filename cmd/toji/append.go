package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mokkori-Tom/toji/internal/cerr"
	"github.com/Mokkori-Tom/toji/internal/pageappend"
	"github.com/Mokkori-Tom/toji/internal/show"
)

// runAppend adds markdown to an existing page:
// `toji append [bundle] <concept-id | file.md> --body <text>|- [--section <heading>] [--dry-run]`.
func runAppend(args []string) {
	var body, section string
	hasBody, dryRun := false, false
	var pos []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--body", "--section":
			if i+1 >= len(args) {
				exitErr(cerr.Usage("%s requires a value", args[i]))
			}
			if args[i] == "--section" {
				section = args[i+1]
			} else {
				body, hasBody = args[i+1], true
			}
			i++
		case "--dry-run":
			dryRun = true
		default:
			if strings.HasPrefix(args[i], "-") {
				exitErr(cerr.Usage("unknown append flag: %s", args[i]))
			}
			pos = append(pos, args[i])
		}
	}
	if len(pos) == 1 {
		pos = []string{defaultBundle(), pos[0]}
	}
	if len(pos) != 2 || !hasBody {
		exitErr(cerr.Usage("usage: toji append [bundle] <concept-id | file.md> --body <text>|- [--section <heading>] [--dry-run]"))
	}
	if body == "-" {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			exitErr(cerr.IO(err, "read body from stdin"))
		}
		body = string(raw)
	}

	var file, id string
	if isMarkdownFile(pos[1]) {
		file, _ = filepath.Abs(pos[1])
		id = strings.TrimSuffix(filepath.Base(file), ".md")
	} else {
		b := mustBundle("append", pos[:1])
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
	out, line, err := pageappend.Append(string(raw), body, section)
	if err != nil {
		var nf *pageappend.NotFoundError
		var amb *pageappend.AmbiguousError
		if errors.As(err, &nf) || errors.As(err, &amb) {
			exitErr(cerr.Validation("%s: %s", id, err))
		}
		exitErr(cerr.Usage("%s", err))
	}
	if !dryRun {
		if err := os.WriteFile(file, []byte(out), 0644); err != nil { //nolint:gosec // the page the user named
			exitErr(cerr.IO(err, "write %s", file))
		}
	}
	if humanOut {
		verb := "appended"
		if dryRun {
			verb = "would append"
		}
		fmt.Fprintf(os.Stderr, "%s at line %d: %s\n", verb, line, file)
		return
	}
	outputJSON(map[string]any{
		"command": "append",
		"id":      id,
		"path":    file,
		"section": section,
		"line":    line,
		"dry_run": dryRun,
	})
}
