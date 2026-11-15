package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Mokkori-Tom/toji/internal/cerr"
	"github.com/Mokkori-Tom/toji/internal/show"
	"github.com/Mokkori-Tom/toji/internal/toc"
)

// runToc writes a table of contents into a page:
// `toji toc [bundle] <concept-id | file.md> [--depth N] [--remove] [--dry-run]`.
func runToc(args []string) {
	var opt toc.Options
	remove, dryRun := false, false
	var pos []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--depth":
			if i+1 >= len(args) {
				exitErr(cerr.Usage("--depth requires a value"))
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > 6 {
				exitErr(cerr.Usage("--depth must be 1 to 6, got %q", args[i+1]))
			}
			opt.Depth = n
			i++
		case "--remove":
			remove = true
		case "--dry-run":
			dryRun = true
		default:
			if strings.HasPrefix(args[i], "-") {
				exitErr(cerr.Usage("unknown toc flag: %s", args[i]))
			}
			pos = append(pos, args[i])
		}
	}
	if len(pos) == 1 {
		pos = []string{defaultBundle(), pos[0]}
	}
	if len(pos) != 2 {
		exitErr(cerr.Usage("usage: toji toc [bundle] <concept-id | file.md> [--depth N] [--remove] [--dry-run]"))
	}
	var file, id string
	if isMarkdownFile(pos[1]) {
		file, _ = filepath.Abs(pos[1])
		id = strings.TrimSuffix(filepath.Base(file), ".md")
	} else {
		b := mustBundle("toc", pos[:1])
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
	text := string(raw)

	var out, status string
	var headings []toc.Heading
	if remove {
		out, status = toc.Remove(text)
	} else {
		headings = toc.Headings(text, opt)
		out, status = toc.Apply(text, toc.Render(headings))
	}
	if status != "unchanged" && !dryRun {
		if err := os.WriteFile(file, []byte(out), 0644); err != nil { //nolint:gosec // the page the user named
			exitErr(cerr.IO(err, "write %s", file))
		}
	}
	if headings == nil {
		headings = []toc.Heading{}
	}
	if humanOut {
		fmt.Print(toc.Render(headings))
		fmt.Fprintf(os.Stderr, "%s: %s\n", status, file)
		return
	}
	outputJSON(map[string]any{
		"command":  "toc",
		"id":       id,
		"path":     file,
		"status":   status,
		"dry_run":  dryRun,
		"headings": headings,
	})
}
