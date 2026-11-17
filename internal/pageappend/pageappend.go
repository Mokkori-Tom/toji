// Package pageappend adds markdown to an existing page without touching what
// is already there: at the end of the page, or at the end of one section.
// It is how an agent, which may not open an editor, adds a related link or a
// follow-up to a page it did not create.
package pageappend

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	headingRE = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*?)[ \t#]*$`)
	fenceRE   = regexp.MustCompile("^\\s{0,3}(```|~~~)")
	// listItemRE is a list item line: "- x", "* x", "+ x", "1. x", "1) x".
	listItemRE = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s`)
)

// NotFoundError is a --section heading the page does not have.
type NotFoundError struct {
	Section  string
	Headings []string // the page's headings, to choose from
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no heading %q in the page (headings: %s)", e.Section, strings.Join(e.Headings, ", "))
}

// AmbiguousError is a --section heading the page has more than once.
type AmbiguousError struct {
	Section string
	Lines   []int // 1-based
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("heading %q appears %d times (lines %v)", e.Section, len(e.Lines), e.Lines)
}

// Append returns text with body added at the end of the page, or, when
// section is not empty, at the end of the section under that heading (before
// the next heading of the same or a higher level). section is the heading's
// text; leading #s are ignored. The added block is separated from what
// precedes and follows it by one blank line. line is the 1-based line where
// body starts in the result.
func Append(text, body, section string) (out string, line int, err error) {
	body = strings.Trim(body, "\n")
	if strings.TrimSpace(body) == "" {
		return "", 0, fmt.Errorf("nothing to append: the body is empty")
	}
	crlf := strings.Contains(text, "\r\n")
	if crlf {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if text == "" {
		lines = nil
	}

	at := len(lines) // insert before this line index
	if section != "" {
		at, err = sectionEnd(lines, section)
		if err != nil {
			return "", 0, err
		}
	}

	// Blank lines at the end of the section stay after the block, as the
	// gap before the next heading.
	before := at
	for before > 0 && strings.TrimSpace(lines[before-1]) == "" {
		before--
	}
	var res []string
	res = append(res, lines[:before]...)
	// A list item added after a list continues it; anything else is set
	// off by a blank line.
	bodyLines := strings.Split(body, "\n")
	continuesList := before > 0 && listItemRE.MatchString(lines[before-1]) && listItemRE.MatchString(bodyLines[0])
	if before > 0 && !continuesList {
		res = append(res, "")
	}
	line = len(res) + 1
	res = append(res, bodyLines...)
	if at < len(lines) {
		res = append(res, "")
		rest := lines[at:]
		for len(rest) > 0 && strings.TrimSpace(rest[0]) == "" {
			rest = rest[1:]
		}
		res = append(res, rest...)
	}
	out = strings.Join(res, "\n") + "\n"
	if crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	return out, line, nil
}

// sectionEnd finds the heading named section and returns the index of the
// line that ends its section: the next heading of the same or a higher
// level, or the end of the page. Headings inside code fences and in the
// frontmatter do not count.
func sectionEnd(lines []string, section string) (int, error) {
	want := normalize(strings.TrimLeft(strings.TrimSpace(section), "#"))
	type heading struct {
		idx, level int
		text       string
	}
	var hs []heading
	inFence := false
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if t := strings.TrimSpace(lines[i]); t == "---" || t == "..." {
				start = i + 1
				break
			}
		}
	}
	for i := start; i < len(lines); i++ {
		if fenceRE.MatchString(lines[i]) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := headingRE.FindStringSubmatch(lines[i]); m != nil {
			hs = append(hs, heading{i, len(m[1]), m[2]})
		}
	}

	found := -1
	var lineNos []int
	for k, h := range hs {
		if normalize(h.text) == want {
			lineNos = append(lineNos, h.idx+1)
			found = k
		}
	}
	switch {
	case len(lineNos) == 0:
		names := make([]string, 0, len(hs))
		for _, h := range hs {
			names = append(names, h.text)
		}
		return 0, &NotFoundError{Section: section, Headings: names}
	case len(lineNos) > 1:
		return 0, &AmbiguousError{Section: section, Lines: lineNos}
	}
	for _, h := range hs[found+1:] {
		if h.level <= hs[found].level {
			return h.idx, nil
		}
	}
	return len(lines), nil
}

// normalize compares headings by their words: case and runs of spaces
// (including the full-width space) do not matter.
func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(s, "　", " ")), " "))
}
