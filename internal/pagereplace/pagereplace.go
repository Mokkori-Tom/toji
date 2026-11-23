// Package pagereplace changes one exact piece of an existing page: it finds
// the old text, which must occur exactly once (in the page or in one
// section), and puts the new text in its place. An empty new text deletes.
// It is how an agent fixes or removes a line of a page without an editor,
// where toji append can only add.
package pagereplace

import (
	"fmt"
	"strings"

	"github.com/Mokkori-Tom/toji/internal/pageappend"
)

// NotFoundError is old text that does not occur where it was looked for.
type NotFoundError struct {
	Section    string // the --section the search was limited to, or ""
	Elsewhere  int    // occurrences outside that section
	SpaceMatch bool   // the text occurs when runs of spaces and line breaks are ignored
}

func (e *NotFoundError) Error() string {
	where := "the page"
	if e.Section != "" {
		where = fmt.Sprintf("section %q", e.Section)
	}
	msg := "the old text does not occur in " + where
	switch {
	case e.Elsewhere > 0:
		msg += fmt.Sprintf(" (it occurs %d time(s) outside the section)", e.Elsewhere)
	case e.SpaceMatch:
		msg += " (it differs only in spaces or line breaks; copy it exactly)"
	}
	return msg
}

// AmbiguousError is old text that occurs more than once without all.
type AmbiguousError struct {
	Lines []int // 1-based line where each occurrence starts
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("the old text occurs %d times (lines %v); add surrounding text to make it unique, limit it with --section, or pass --all", len(e.Lines), e.Lines)
}

// Replace returns text with old replaced by new. old must occur exactly once
// in the page, or in the section under the heading named section (the
// heading line and the section's body, up to the next heading of the same or
// a higher level); with all, every occurrence there is replaced. Matching is
// exact, except that CRLF line endings are treated as LF. When new is empty
// and an occurrence covers whole lines, its line break is removed too, so no
// blank line is left in its place. lines are the 1-based lines of the input
// where the occurrences start.
func Replace(text, old, new, section string, all bool) (out string, lines []int, err error) {
	crlf := strings.Contains(text, "\r\n")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	old = strings.ReplaceAll(old, "\r\n", "\n")
	new = strings.ReplaceAll(new, "\r\n", "\n")
	if strings.TrimSpace(old) == "" {
		return "", nil, fmt.Errorf("nothing to replace: the old text is empty")
	}

	lo, hi := 0, len(text) // byte range searched
	if section != "" {
		pageLines := strings.Split(text, "\n")
		start, end, err := pageappend.SectionBounds(pageLines, section)
		if err != nil {
			return "", nil, err
		}
		lo = offsetOfLine(pageLines, start)
		hi = offsetOfLine(pageLines, end)
	}

	var at []int // byte offsets of the occurrences inside [lo, hi)
	for i := lo; i <= hi-len(old); {
		j := strings.Index(text[i:hi], old)
		if j < 0 {
			break
		}
		at = append(at, i+j)
		i += j + len(old)
	}
	switch {
	case len(at) == 0:
		nf := &NotFoundError{Section: section}
		if section != "" {
			nf.Elsewhere = strings.Count(text, old)
		}
		if nf.Elsewhere == 0 && strings.Contains(squeeze(text[lo:hi]), squeeze(old)) {
			nf.SpaceMatch = true
		}
		return "", nil, nf
	case len(at) > 1 && !all:
		ls := make([]int, len(at))
		for k, a := range at {
			ls[k] = lineAt(text, a)
		}
		return "", nil, &AmbiguousError{Lines: ls}
	}

	var b strings.Builder
	prev := 0
	for _, a := range at {
		lines = append(lines, lineAt(text, a))
		end := a + len(old)
		// Deleting whole lines takes their line break with them.
		if new == "" && (a == 0 || text[a-1] == '\n') && !strings.HasSuffix(old, "\n") && end < len(text) && text[end] == '\n' {
			end++
		}
		b.WriteString(text[prev:a])
		b.WriteString(new)
		prev = end
	}
	b.WriteString(text[prev:])
	out = b.String()
	if crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	return out, lines, nil
}

// offsetOfLine is the byte offset where line index i starts (len of the text
// for i past the last line).
func offsetOfLine(lines []string, i int) int {
	off := 0
	for k := 0; k < i && k < len(lines); k++ {
		off += len(lines[k]) + 1
	}
	if total := len(strings.Join(lines, "\n")); off > total {
		return total
	}
	return off
}

// lineAt is the 1-based line of byte offset off.
func lineAt(text string, off int) int {
	return strings.Count(text[:off], "\n") + 1
}

// squeeze collapses runs of whitespace (spaces, tabs, line breaks, the
// full-width space) to one space, to tell "differs only in spacing" apart.
func squeeze(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "　", " ")), " ")
}
