// Package search filters concepts in a bundle by tag, type, or body text.
package search

import (
	"os"
	"path"
	"sort"
	"strings"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/concept"
	"github.com/okfcli/okf/internal/validate"
)

// Filters holds optional filter criteria. Empty fields match everything.
type Filters struct {
	Tag   string   // match if concept has this tag
	Type  string   // match if concept frontmatter type equals this
	Text  string   // match if text appears in title, description, or body (case-insensitive)
	Terms []string // match if every term appears in title, description, or body; split on whitespace
	// Meta limits text matching to the title, description, tags and the
	// index.md entries that link to the concept; the body is not searched.
	Meta bool `json:",omitempty"`
}

// Search returns concepts matching all provided filters.
// Empty filters return all concepts.
func Search(b *bundle.Bundle, f Filters) []*concept.Concept {
	terms := f.terms()
	var entries map[string]string
	if f.Meta && len(terms) > 0 {
		entries = IndexEntries(b)
	}
	var results []*concept.Concept
	for _, c := range b.Concepts {
		if !matchTag(c, f.Tag) {
			continue
		}
		if !matchType(c, f.Type) {
			continue
		}
		if f.Meta {
			if !containsAll(Fold(metaText(c, entries[c.ID])), terms) {
				continue
			}
		} else if !matchTerms(c, terms) {
			continue
		}
		results = append(results, c)
	}
	return results
}

// Line is one body line that contains at least one search term.
type Line struct {
	Number int    `json:"line"` // 1-based line number in the file
	Text   string `json:"text"`
}

// Hit is a search result with the lines that matched.
type Hit struct {
	Concept *concept.Concept
	Lines   []Line
	// Summary is set in Meta mode: the frontmatter description, else the
	// text of the index.md entry that links to the concept.
	Summary string
	inTitle bool
}

// Find runs Search and, when text terms are given, attaches the matching
// lines and ranks results: concepts whose title holds every term first, then
// by number of matching lines, then by ID. Without terms the ID order of
// Search is kept. In Meta mode no file is read again: each hit carries a
// Summary instead of matching lines.
func Find(b *bundle.Bundle, f Filters) []Hit {
	terms := f.terms()
	results := Search(b, f)
	var entries map[string]string
	if f.Meta {
		entries = IndexEntries(b)
	}
	hits := make([]Hit, 0, len(results))
	for _, c := range results {
		h := Hit{Concept: c}
		if f.Meta {
			h.Summary = c.Frontmatter.Description
			if h.Summary == "" {
				h.Summary = entries[c.ID]
			}
			if len(terms) > 0 {
				h.inTitle = containsAll(Fold(c.DisplayTitle()), terms)
			}
		} else if len(terms) > 0 {
			h.Lines = matchLines(c.Path, terms)
			h.inTitle = containsAll(Fold(c.DisplayTitle()), terms)
		}
		hits = append(hits, h)
	}
	if len(terms) > 0 {
		sort.SliceStable(hits, func(i, j int) bool {
			if hits[i].inTitle != hits[j].inTitle {
				return hits[i].inTitle
			}
			return len(hits[i].Lines) > len(hits[j].Lines)
		})
	}
	return hits
}

// terms returns the folded, non-empty text terms used for matching. Text is
// one phrase, as upstream. Each entry of Terms is split on whitespace,
// including the full-width space a Japanese IME types, so "ファン　電力"
// passed as one argument still means two terms.
func (f Filters) terms() []string {
	var out []string
	if t := strings.TrimSpace(Fold(f.Text)); t != "" {
		out = append(out, t)
	}
	for _, t := range f.Terms {
		out = append(out, strings.Fields(Fold(t))...)
	}
	return out
}

// FoldedTerms is the exported form of terms, for callers that highlight hits.
func (f Filters) FoldedTerms() []string { return f.terms() }

func matchTag(c *concept.Concept, tag string) bool {
	if tag == "" {
		return true
	}
	for _, t := range c.Frontmatter.Tags {
		if Fold(t) == Fold(tag) {
			return true
		}
	}
	return false
}

func matchType(c *concept.Concept, typ string) bool {
	if typ == "" {
		return true
	}
	return strings.EqualFold(c.Frontmatter.Type, typ)
}

// matchTerms reports whether every term appears in the title, description,
// or body. Terms are already folded.
func matchTerms(c *concept.Concept, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	fm := c.Frontmatter
	return containsAll(Fold(c.DisplayTitle()+"\n"+fm.Description+"\n"+c.Body), terms)
}

// metaText is what Meta mode matches against: title, description, tags and
// the concept's index.md entry.
func metaText(c *concept.Concept, entry string) string {
	fm := c.Frontmatter
	return c.DisplayTitle() + "\n" + fm.Description + "\n" + strings.Join(fm.Tags, "\n") + "\n" + entry
}

// IndexEntries maps concept IDs to the text that the bundle's index.md files
// write beside their links, "- [Title](./page.md) - one-line summary"
// giving "one-line summary". Links into folders and to missing pages are
// skipped; when several entries link to one page, the first non-empty wins.
func IndexEntries(b *bundle.Bundle) map[string]string {
	out := make(map[string]string)
	for _, r := range b.Reserved {
		if !strings.EqualFold(path.Base(r.ID), "index") {
			continue
		}
		for _, line := range strings.Split(r.Body, "\n") {
			links := validate.ExtractLinks(line)
			if len(links) != 1 || links[0].Wiki {
				continue
			}
			id := validate.ResolveLink(r.ID, links[0])
			if id == "" || !b.HasConcept(id) || out[id] != "" {
				continue
			}
			out[id] = entryText(line, links[0].Target)
		}
	}
	return out
}

// entryText returns what follows the link to target on an index line, with
// the " - " or ": " separator removed.
func entryText(line, target string) string {
	_, after, ok := strings.Cut(line, "("+target+")")
	if !ok {
		return ""
	}
	// Table rows from okf index: "| [Title](page.md) | Type | Summary |".
	after = strings.Trim(strings.TrimSpace(after), "| ")
	for _, sep := range []string{"-", "–", "—", ":", "："} {
		if rest, ok := strings.CutPrefix(after, sep); ok {
			return strings.TrimSpace(rest)
		}
	}
	return after
}

func containsAll(folded string, terms []string) bool {
	for _, t := range terms {
		if !strings.Contains(folded, t) {
			return false
		}
	}
	return true
}

// matchLines returns the body lines of the file at path that contain any of
// the folded terms, numbered as in the file so an editor can jump to them.
func matchLines(path string, terms []string) []Line {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimPrefix(string(raw), "\uFEFF"), "\n")
	start := 0
	if len(lines) > 0 && strings.TrimRight(lines[0], "\r") == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimRight(lines[i], "\r") == "---" {
				start = i + 1
				break
			}
		}
	}
	var out []Line
	for i := start; i < len(lines); i++ {
		text := strings.TrimRight(lines[i], "\r")
		folded := Fold(text)
		for _, t := range terms {
			if strings.Contains(folded, t) {
				out = append(out, Line{Number: i + 1, Text: text})
				break
			}
		}
	}
	return out
}
