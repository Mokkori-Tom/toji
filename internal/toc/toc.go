// Package toc writes a table of contents into a page: a list of links to
// its headings, kept between <!-- toc --> and <!-- /toc --> markers so it
// can be rebuilt after the page changes, as the Markdown All in One
// extension for VS Code does.
package toc

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Options controls which headings are listed.
type Options struct {
	// Depth is the deepest heading level listed. 0 means 3.
	Depth int
}

// Heading is one heading of the page.
type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	Slug  string `json:"slug"`
	Line  int    `json:"line"` // 1-based, in the whole file
}

// Markers that hold the table of contents. The closing marker also accepts
// the form Markdown All in One writes.
var (
	openRE  = regexp.MustCompile(`(?i)^\s*<!--\s*toc\s*-->\s*$`)
	closeRE = regexp.MustCompile(`(?i)^\s*<!--\s*(/toc|tocstop)\s*-->\s*$`)
	omitRE  = regexp.MustCompile(`(?i)<!--\s*omit\s+(in|from)\s+toc\s*-->`)
	fenceRE = regexp.MustCompile("^\\s{0,3}(```|~~~)")
)

// The heading of the table itself is not listed in it.
var tocTitles = map[string]bool{"目次": true, "table of contents": true, "contents": true, "toc": true}

// Headings lists the page's headings that belong in its table of contents.
// A single level-1 heading is the page title and is left out; when the page
// uses several level-1 headings as sections they are listed. Headings inside
// code fences, marked <!-- omit in toc -->, or titling the table itself are
// skipped. text is the whole file; headings in the frontmatter do not exist.
func Headings(text string, opt Options) []Heading {
	depth := opt.Depth
	if depth <= 0 {
		depth = 3
	}
	lines := strings.Split(text, "\n")
	start := frontmatterEnd(lines)

	var all []Heading
	inFence := false
	fence := ""
	for i := start; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if m := fenceRE.FindStringSubmatch(line); m != nil {
			switch {
			case !inFence:
				inFence, fence = true, m[1]
			case strings.HasPrefix(strings.TrimSpace(line), fence):
				inFence = false
			}
			continue
		}
		if inFence {
			continue
		}
		level, txt, ok := parseHeading(line)
		if !ok || omitRE.MatchString(txt) || tocTitles[strings.ToLower(strings.TrimSpace(txt))] {
			continue
		}
		all = append(all, Heading{Level: level, Text: cleanText(txt), Line: i + 1})
	}

	// One H1 is the title; several are sections.
	minLevel := 2
	h1 := 0
	for _, h := range all {
		if h.Level == 1 {
			h1++
		}
	}
	if h1 > 1 {
		minLevel = 1
	}
	var out []Heading
	seen := map[string]int{}
	for _, h := range all {
		if h.Level < minLevel || h.Level > depth {
			continue
		}
		h.Slug = uniqueSlug(slug(h.Text), seen)
		out = append(out, h)
	}
	return out
}

// Render writes the list, indented two spaces per level under the first.
func Render(hs []Heading) string {
	if len(hs) == 0 {
		return ""
	}
	min := hs[0].Level
	for _, h := range hs {
		if h.Level < min {
			min = h.Level
		}
	}
	var sb strings.Builder
	for _, h := range hs {
		sb.WriteString(strings.Repeat("  ", h.Level-min))
		sb.WriteString("- [" + h.Text + "](#" + h.Slug + ")\n")
	}
	return sb.String()
}

// Apply puts list between the page's markers, adding them under the title
// when the page has none, and returns the new text and what happened:
// inserted, updated or unchanged. An empty list with no markers leaves the
// page alone.
func Apply(text, list string) (string, string) {
	lines := strings.Split(text, "\n")
	open, close := findMarkers(lines)
	block := "<!-- toc -->\n" + list + "<!-- /toc -->"
	if open == -1 {
		if list == "" {
			return text, "unchanged"
		}
		at := insertAt(lines)
		out := append([]string{}, lines[:at]...)
		if at > 0 && strings.TrimSpace(lines[at-1]) != "" {
			out = append(out, "")
		}
		out = append(out, strings.Split(block, "\n")...)
		rest := lines[at:]
		if len(rest) > 0 && strings.TrimSpace(rest[0]) != "" {
			out = append(out, "")
		}
		out = append(out, rest...)
		return strings.Join(out, "\n"), "inserted"
	}
	out := append([]string{}, lines[:open]...)
	out = append(out, strings.Split(block, "\n")...)
	out = append(out, lines[close+1:]...)
	joined := strings.Join(out, "\n")
	if joined == text {
		return text, "unchanged"
	}
	return joined, "updated"
}

// Remove takes the markers and the list between them out of the page.
func Remove(text string) (string, string) {
	lines := strings.Split(text, "\n")
	open, close := findMarkers(lines)
	if open == -1 {
		return text, "unchanged"
	}
	end := close + 1
	// The blank line that separated the block from what follows goes too.
	if end < len(lines) && strings.TrimSpace(lines[end]) == "" && open > 0 && strings.TrimSpace(lines[open-1]) == "" {
		end++
	}
	out := append([]string{}, lines[:open]...)
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n"), "removed"
}

// findMarkers returns the line indexes of the opening and closing markers,
// or -1, -1. A closing marker is required; an opening one on its own is
// ignored so a stray comment does not swallow the page.
func findMarkers(lines []string) (int, int) {
	open := -1
	for i, l := range lines {
		switch {
		case open == -1 && openRE.MatchString(l):
			open = i
		case open != -1 && closeRE.MatchString(l):
			return open, i
		}
	}
	return -1, -1
}

// insertAt is where a new block goes: after the title heading and the
// blank line under it, else at the top of the body.
func insertAt(lines []string) int {
	at := frontmatterEnd(lines)
	for at < len(lines) && strings.TrimSpace(lines[at]) == "" {
		at++
	}
	if at < len(lines) {
		if level, _, ok := parseHeading(strings.TrimRight(lines[at], "\r")); ok && level == 1 {
			at++
			for at < len(lines) && strings.TrimSpace(lines[at]) == "" {
				at++
			}
			return at
		}
	}
	return at
}

// frontmatterEnd is the index of the first body line.
func frontmatterEnd(lines []string) int {
	if len(lines) == 0 || strings.TrimRight(strings.TrimPrefix(lines[0], "\uFEFF"), "\r") != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			return i + 1
		}
	}
	return 0
}

// parseHeading reads an ATX heading: 1-6 # then a space, closing #s optional.
func parseHeading(line string) (int, string, bool) {
	s := strings.TrimLeft(line, " ")
	if len(line)-len(s) > 3 {
		return 0, "", false
	}
	level := 0
	for level < len(s) && s[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level == len(s) || (s[level] != ' ' && s[level] != '\t') {
		return 0, "", false
	}
	txt := strings.TrimSpace(s[level:])
	txt = strings.TrimRight(txt, "#")
	return level, strings.TrimSpace(txt), true
}

var (
	linkRE = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	wikiRE = regexp.MustCompile(`\[\[(?:[^|\]]*\|)?([^\]]*)\]\]`)
	htmlRE = regexp.MustCompile(`<[^>]+>`)
)

// cleanText is the heading as link text: links reduced to their text, HTML
// comments and tags removed.
func cleanText(s string) string {
	s = linkRE.ReplaceAllString(s, "$1")
	s = wikiRE.ReplaceAllString(s, "$1")
	s = htmlRE.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// slug makes the fragment GitHub (and mdBook, and okf-export) gives a
// heading: lower case, punctuation dropped, spaces to hyphens, letters of
// any script kept.
func slug(text string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(text)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-':
			sb.WriteRune(r)
		case r == ' ':
			sb.WriteRune('-')
		}
	}
	if sb.Len() == 0 {
		return "heading"
	}
	return sb.String()
}

func uniqueSlug(s string, seen map[string]int) string {
	n := seen[s]
	seen[s]++
	if n == 0 {
		return s
	}
	return s + "-" + strconv.Itoa(n)
}
