// Package index writes index.md files for the directories of a bundle. Per
// OKF spec §6, index.md is a directory listing for progressive disclosure:
// agents and humans navigate one level at a time.
//
// An index is part written by hand and part derived, and Generate keeps the
// two apart. Hand-written: the heading, the paragraph under it (what the
// folder is for), and any section that is not a plain list of links. Derived
// and rebuilt on every run: the list of folders, the list of pages, and the
// list of other files, each entry "- [name](./target) - summary". Any list
// Markdown allows is read ("* ", "+ ", "1. "), and a list keeps the marker,
// the separator and the link form it was written with. A page's
// summary is its frontmatter description; a folder's is the paragraph at the
// top of its own index.md; where neither exists the summary already in the
// index is kept, so nothing written by hand is lost.
package index

import (
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/Mokkori-Tom/toji/internal/concept"
	"github.com/Mokkori-Tom/toji/internal/textdiff"
)

// Options controls Generate.
type Options struct {
	DryRun           bool     // report what would change without writing
	FillDescriptions bool     // give pages without a frontmatter description the summary their index carries
	Dirs             []string // bundle-relative folders to update ("." is the root); empty means every folder
	Diff             bool     // fill IndexResult.Diff with a unified diff of each change
}

// Result reports what Generate did, or with DryRun would do.
type Result struct {
	Indexes []IndexResult
	Filled  []string // bundle-relative paths of pages given a description
}

// IndexResult describes one index.md.
type IndexResult struct {
	Path           string // absolute
	Dir            string // bundle-relative, "." for the root
	Status         string // created, updated, unchanged
	HasDescription bool   // a paragraph under the heading describes the folder
	Entries        int
	Diff           string // unified diff from the old index to the new, with Options.Diff
}

// Generate updates index.md in every directory of root that has pages, folders
// of pages, or an index.md already, and returns what changed. With opt.Dirs
// only those folders are updated, each on its own (not its subfolders).
func Generate(root string, opt Options) (*Result, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	// Follow a symlinked root; WalkDir would not descend into it.
	if real, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = real
	}
	g := &generator{root: absRoot, opt: opt, names: detectNames(absRoot)}
	res := &Result{}
	if len(opt.Dirs) > 0 {
		for _, d := range opt.Dirs {
			dir, err := g.resolveDir(d)
			if err != nil {
				return nil, err
			}
			if err := g.dir(dir, res); err != nil {
				return nil, err
			}
		}
		return res, nil
	}
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != absRoot && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		return g.dir(path, res)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// resolveDir turns a bundle-relative folder into an absolute path inside the
// bundle, refusing one that is missing, not a folder, or outside the bundle.
func (g *generator) resolveDir(d string) (string, error) {
	rel := filepath.Clean(filepath.FromSlash(strings.TrimSuffix(d, "/")))
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", &DirError{Dir: d, Reason: "is not inside the bundle"}
	}
	dir := filepath.Join(g.root, rel)
	info, err := os.Stat(dir)
	if err != nil {
		return "", &DirError{Dir: d, Reason: "does not exist"}
	}
	if !info.IsDir() {
		return "", &DirError{Dir: d, Reason: "is not a folder"}
	}
	return dir, nil
}

// DirError is an Options.Dirs entry that cannot be indexed.
type DirError struct {
	Dir    string
	Reason string
}

func (e *DirError) Error() string { return "folder " + e.Dir + " " + e.Reason }

// role is what a list section holds.
type role int

const (
	roleOther role = iota // kept as written
	roleFolder
	roleDoc
	roleFile
)

// names are the heading lines ("## Folders") of the three list sections. Taken from the indexes
// the bundle already has, so a bundle that says "## フォルダ" keeps saying it.
type names map[role]string

var defaultNames = names{roleFolder: "## Folders", roleDoc: "## Documents", roleFile: "## Files"}

// detectNames reads every index.md and, for each role, takes the heading most
// often used over a section whose entries all have that role.
func detectNames(root string) names {
	count := map[role]map[string]int{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(d.Name()) != "index.md" {
			return nil
		}
		doc, err := parseIndex(path)
		if err != nil {
			return nil
		}
		for _, s := range doc.sections {
			if r := s.uniformRole(filepath.Dir(path)); r != roleOther && s.heading != "" {
				if count[r] == nil {
					count[r] = map[string]int{}
				}
				count[r][s.heading]++
			}
		}
		return nil
	})
	out := names{}
	for r, def := range defaultNames {
		best, n := def, 0
		for h, c := range count[r] {
			if c > n || (c == n && h < best) {
				best, n = h, c
			}
		}
		out[r] = best
	}
	return out
}

type generator struct {
	root  string
	opt   Options
	names names
}

// listing is what a directory holds, as the index should show it.
type listing struct {
	folders []string // subdirectory names
	pages   []string // page file names, .md
	files   []string // other file names
}

func (g *generator) dir(dir string, res *Result) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var l listing
	hasIndex := false
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		switch {
		case e.IsDir():
			if isListedFolder(filepath.Join(dir, name)) {
				l.folders = append(l.folders, name)
			}
		case concept.ReservedNames[strings.ToLower(name)]:
			if strings.ToLower(name) == "index.md" {
				hasIndex = true
			}
		case strings.HasSuffix(name, ".md"):
			l.pages = append(l.pages, name)
		default:
			l.files = append(l.files, name)
		}
	}
	if !hasIndex && len(l.pages) == 0 && len(l.folders) == 0 {
		return nil // attachments only, and nobody wrote an index for them
	}
	sortNames(l.folders)
	sortNames(l.pages)
	sortNames(l.files)

	indexPath := filepath.Join(dir, "index.md")
	var old *indexDoc
	var oldText string
	if hasIndex {
		raw, err := os.ReadFile(indexPath)
		if err != nil {
			return err
		}
		oldText = string(raw)
		old = parseIndexText(oldText)
	}

	rel, _ := filepath.Rel(g.root, dir)
	rel = filepath.ToSlash(rel)
	text, ir := g.render(dir, rel, old, l)
	ir.Path = indexPath
	ir.Dir = rel
	switch {
	case !hasIndex:
		ir.Status = "created"
	case strings.TrimRight(text, "\n") == strings.TrimRight(oldText, "\n"):
		// Only the final newline differs: not worth a write.
		text = oldText
		ir.Status = "unchanged"
	default:
		ir.Status = "updated"
	}
	if g.opt.Diff && ir.Status != "unchanged" {
		ir.Diff = textdiff.UnifiedDiff(oldText, text, filepath.ToSlash(filepath.Join(rel, "index.md")))
	}
	if ir.Status != "unchanged" && !g.opt.DryRun {
		if err := os.WriteFile(indexPath, []byte(text), 0644); err != nil { //nolint:gosec // the bundle's own index.md
			return err
		}
	}
	res.Indexes = append(res.Indexes, ir)

	if g.opt.FillDescriptions && old != nil {
		for _, s := range old.sections {
			if s.listRole(dir, g.names) != roleDoc {
				continue
			}
			for _, e := range s.entries {
				if e.role(dir) != roleDoc || e.desc == "" || strings.HasSuffix(e.desc, "…") || strings.HasSuffix(e.desc, "...") {
					continue
				}
				p := filepath.Join(dir, e.name())
				filled, err := fillDescription(p, e.desc, g.opt.DryRun)
				if err != nil {
					return err
				}
				if filled {
					relPage, _ := filepath.Rel(g.root, p)
					res.Filled = append(res.Filled, filepath.ToSlash(relPage))
				}
			}
		}
	}
	return nil
}

// render writes the index for dir from the old one, if any, and the listing.
func (g *generator) render(dir, rel string, old *indexDoc, l listing) (string, IndexResult) {
	var sb strings.Builder
	var ir IndexResult

	// Frontmatter is kept as it was (a root index may declare okf_version,
	// §12) and never added.
	head := []string{"# " + filepath.Base(dir), ""}
	if rel == "." {
		head[0] = "# Index"
	}
	if old != nil {
		sb.WriteString(old.front)
		if len(old.head) > 0 || (len(old.sections) > 0 && old.sections[0].heading == "") {
			head = old.head
		}
	}
	ir.HasDescription = introParagraph(head) != ""
	for _, line := range trimBlank(head) {
		sb.WriteString(line + "\n")
	}

	// Summaries already in the index, by target, so a hand-written one
	// survives until a frontmatter description or a folder's own index
	// replaces it. Targets linked from a hand-written section are not
	// listed again.
	oldDesc := map[string]string{}
	oldEntry := map[string]entry{}
	pinned := map[string]bool{}
	done := map[role]bool{}
	oldOrder := map[role][]string{}
	pin := func(e entry) {
		pinned[e.name()] = true
		// "bin/README.md" stands for the folder bin.
		if first, _, ok := strings.Cut(e.name(), "/"); ok && !strings.Contains(e.target, "://") {
			pinned[first] = true
		}
	}
	if old != nil {
		// Links in the text under the title, a list mixing links and
		// plain items included, are written by hand.
		for _, line := range old.head {
			if e, ok := parseEntry(line); ok {
				pin(e)
			}
		}
		for _, s := range old.sections {
			r := s.listRole(dir, g.names)
			for _, e := range s.entries {
				if e.desc != "" {
					oldDesc[e.name()] = e.desc
				}
				oldEntry[e.name()] = e
				if r == roleOther {
					pin(e)
				} else {
					oldOrder[r] = append(oldOrder[r], e.name())
				}
			}
		}
	}
	// A summary that only changed in whitespace is left as written.
	desc := func(name, fresh string) string {
		if o := oldDesc[name]; fresh == "" || strings.Join(strings.Fields(o), " ") == fresh {
			return o
		}
		return fresh
	}
	// A link already in the index keeps its text and target as written
	// ("GTDandOKF/" stays, not "./GTDandOKF/"), so a run changes only what
	// the folder's contents changed.
	link := func(name string, fresh entry) entry {
		if o, ok := oldEntry[name]; ok {
			fresh.pad, fresh.text, fresh.target, fresh.angle, fresh.title = o.pad, o.text, o.target, o.angle, o.title
			if o.sep != "" {
				fresh.sep = o.sep
			}
		}
		return fresh
	}
	build := func(r role) []entry {
		var out []entry
		switch r {
		case roleFolder:
			for _, f := range keepOrder(l.folders, oldOrder[r]) {
				if pinned[f] {
					continue
				}
				d := desc(f, folderSummary(filepath.Join(dir, f, "index.md")))
				e := link(f, entry{text: f + "/", target: "./" + escape(f) + "/", desc: d})
				// A folder link points at its index.md, not the folder: a
				// directory is no page to open. Without --dir every listed
				// folder gets an index.md in the same run.
				if strings.HasSuffix(e.target, "/") && (len(g.opt.Dirs) == 0 || fileExists(filepath.Join(dir, f, "index.md"))) {
					e.target += "index.md"
				}
				out = append(out, e)
			}
		case roleDoc:
			for _, p := range keepOrder(l.pages, oldOrder[r]) {
				if pinned[p] {
					continue
				}
				d := desc(p, pageDescription(filepath.Join(dir, p)))
				out = append(out, link(p, entry{text: strings.TrimSuffix(p, ".md"), target: "./" + escape(p), desc: d}))
			}
		case roleFile:
			for _, f := range keepOrder(l.files, oldOrder[r]) {
				if pinned[f] {
					continue
				}
				out = append(out, link(f, entry{text: f, target: "./" + escape(f), desc: oldDesc[f]}))
			}
		}
		return out
	}
	// A section is written as its list already was: the marker ("-", "*",
	// "+", or numbers counted on from the first), and the blank lines
	// around the heading and between items, all of which Markdown allows.
	writeSection := func(heading string, es []entry, like *section) {
		if len(es) == 0 {
			return
		}
		if like == nil {
			like = &section{}
		}
		if !like.tightBefore {
			sb.WriteString("\n")
		}
		if heading != "" {
			sb.WriteString(heading + "\n")
			if !like.tightHeading {
				sb.WriteString("\n")
			}
		}
		mark := func(int) string { return "-" }
		pad := ""
		if len(like.entries) > 0 {
			m := like.entries[0].marker
			pad = like.entries[0].pad
			if n, err := strconv.Atoi(m[:len(m)-1]); err == nil {
				delim := m[len(m)-1:]
				mark = func(i int) string { return strconv.Itoa(n+i) + delim }
			} else {
				mark = func(int) string { return m }
			}
		}
		for i, e := range es {
			if i > 0 && like.loose {
				sb.WriteString("\n")
			}
			e.marker = mark(i)
			if e.pad == "" {
				e.pad = pad
			}
			sb.WriteString(e.String() + "\n")
		}
		ir.Entries += len(es)
	}

	// Sections in their old order: list sections rebuilt, others verbatim.
	if old != nil {
		for _, s := range old.sections {
			r := s.listRole(dir, g.names)
			if r == roleOther || done[r] {
				lines := trimBlank(s.lines)
				if s.heading == "" && len(lines) == 0 {
					continue
				}
				if !s.tightBefore {
					sb.WriteString("\n")
				}
				if s.heading != "" {
					sb.WriteString(s.heading + "\n")
					if len(lines) > 0 && !s.tightHeading {
						sb.WriteString("\n")
					}
				}
				for _, line := range lines {
					sb.WriteString(line + "\n")
				}
				continue
			}
			done[r] = true
			writeSection(s.heading, build(r), s)
		}
	}
	for _, r := range []role{roleFolder, roleDoc, roleFile} {
		if !done[r] {
			writeSection(g.names[r], build(r), nil)
		}
	}
	return sb.String(), ir
}

// --- the index file ---

type indexDoc struct {
	front    string   // frontmatter block with its delimiters and trailing newline, or ""
	head     []string // lines before the first "## " heading
	sections []*section
}

type section struct {
	heading string   // the "## ..." line; "" for a list right under the title
	lines   []string // every line under it, as written
	entries []entry  // the link entries among them
	plain   bool     // every non-blank line is an entry

	tightBefore  bool // no blank line between the text above and this section
	tightHeading bool // no blank line between the heading and the list
	loose        bool // blank lines between the items
}

// listRole is the role a section is rebuilt as. A section whose entries all
// point at the same kind of thing has that role (uniformRole). A section under
// the bundle's own heading for a list ("## 文書") keeps that role when a few of
// its entries point at another kind of thing in this folder, such as a file
// link among the pages: those entries move to their own list, which is where
// toji puts them anyway. A link that is not to something in this folder (a URL,
// another folder's page) still keeps the whole section as written, so nothing
// is ever dropped.
func (s *section) listRole(dir string, n names) role {
	if r := s.uniformRole(dir); r != roleOther || !s.plain || s.heading == "" {
		return r
	}
	for _, r := range []role{roleFolder, roleDoc, roleFile} {
		if s.heading != n[r] {
			continue
		}
		for _, e := range s.entries {
			if e.role(dir) == roleOther {
				return roleOther
			}
		}
		return r
	}
	return roleOther
}

// uniformRole is the role of a section whose entries all point at the same
// kind of thing, resolved against dir; a section with no entries, with mixed
// targets, or with text other than entries is roleOther and kept as written.
func (s *section) uniformRole(dir string) role {
	if !s.plain || len(s.entries) == 0 {
		return roleOther
	}
	r := roleOther
	for _, e := range s.entries {
		er := e.role(dir)
		if er == roleOther || (r != roleOther && er != r) {
			return roleOther
		}
		r = er
	}
	return r
}

type entry struct {
	marker string // "-", "*", "+", "1." or "1)"
	pad    string // the spaces after the marker, " " if new
	text   string
	target string
	angle  bool   // the target was written as <target>
	title  string // a link title as written, quotes included
	sep    string // what stands between the link and the summary, " - " if new
	desc   string
}

// entryRE matches a list item that is one link and an optional summary, in
// any form CommonMark allows for a top-level item: "-", "*", "+" or an
// ordered "1." / "1)" marker, brackets nested once in the text, a target in
// <angle brackets> or with balanced parentheses, and a link title.
var entryRE = regexp.MustCompile(`^([-*+]|\d{1,9}[.)])([ \t]+)\[((?:[^\[\]]|\[[^\[\]]*\])*)\]\(\s*(?:<([^<>\n]*)>|((?:[^()\s]|\([^()\s]*\))+))(?:\s+("[^"]*"|'[^']*'|\([^()]*\)))?\s*\)(?:(\s*[-–—:：]\s*)(.*))?$`)

func parseEntry(line string) (entry, bool) {
	m := entryRE.FindStringSubmatch(strings.TrimRight(line, " \t"))
	if m == nil {
		return entry{}, false
	}
	e := entry{marker: m[1], pad: m[2], text: m[3], target: m[5], title: m[6], desc: strings.TrimSpace(m[8])}
	if m[4] != "" || m[5] == "" {
		e.target, e.angle = m[4], true
	}
	if e.desc != "" {
		e.sep = m[7]
	}
	return e, true
}

func (e entry) String() string {
	marker := e.marker
	if marker == "" {
		marker = "-"
	}
	target := e.target
	if e.angle {
		target = "<" + target + ">"
	}
	if e.title != "" {
		target += " " + e.title
	}
	pad := e.pad
	if pad == "" {
		pad = " "
	}
	s := marker + pad + "[" + e.text + "](" + target + ")"
	if e.desc != "" {
		sep := e.sep
		if sep == "" {
			sep = " - "
		}
		s += sep + e.desc
	}
	return s
}

// name is the file or folder the entry points at, within its directory.
func (e entry) name() string {
	t := unescape(e.target)
	t = strings.TrimPrefix(t, "./")
	if i := strings.IndexAny(t, "#?"); i != -1 {
		t = t[:i]
	}
	t = strings.TrimSuffix(t, "/index.md")
	t = strings.TrimSuffix(t, "/")
	return t
}

// role classifies the entry by what it points at in dir: a folder, a page
// (an .md that is not reserved), another file, or something that is not
// there (another folder's page, a URL), which keeps its section as written.
func (e entry) role(dir string) role {
	n := e.name()
	if n == "" || strings.Contains(n, "/") || strings.Contains(e.target, "://") {
		return roleOther
	}
	if concept.ReservedNames[strings.ToLower(n)] {
		return roleOther
	}
	info, err := os.Stat(filepath.Join(dir, n)) //nolint:gosec // a link target inside the bundle
	if err != nil {
		// Gone from the folder: the entry is dropped, and the kind of its
		// target tells which list it was in.
		switch {
		case strings.HasSuffix(e.target, "/") || strings.HasSuffix(e.target, "/index.md"):
			return roleFolder
		case strings.HasSuffix(n, ".md"):
			return roleDoc
		}
		return roleFile
	}
	switch {
	case info.IsDir():
		return roleFolder
	case strings.HasSuffix(n, ".md"):
		return roleDoc
	}
	return roleFile
}

func parseIndex(path string) (*indexDoc, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseIndexText(string(raw)), nil
}

func parseIndexText(text string) *indexDoc {
	text = strings.TrimPrefix(text, "\uFEFF")
	orig := text
	doc := &indexDoc{}
	if front, body, ok := concept.SplitFrontmatter(text); ok {
		doc.front = "---\n" + front + "\n---\n"
		text = body
		// Blank lines between the frontmatter and the title stay as written
		// (SplitFrontmatter drops one).
		if block := doc.front; strings.HasPrefix(orig, block) {
			rest := orig[len(block):]
			for strings.HasPrefix(rest, "\n") {
				doc.front += "\n"
				rest = rest[1:]
			}
			text = rest
		}
	}
	// The first heading is the title; every heading after it, of any
	// level, starts a section (OKF §8 writes "# Another section"). A "#"
	// line inside a fenced code block is not a heading.
	var cur *section
	titled, fence := false, ""
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		line = strings.TrimRight(line, "\r")
		if f := fenceRE.FindString(line); f != "" {
			f = strings.TrimLeft(f, " ")
			switch {
			case fence == "":
				fence = f[:3]
			case strings.HasPrefix(f, fence):
				fence = ""
			}
		} else if fence == "" && headingRE.MatchString(line) && (titled || cur != nil) {
			above := doc.head
			if cur != nil {
				above = cur.lines
			}
			cur = &section{heading: line, plain: true}
			cur.tightBefore = len(above) > 0 && strings.TrimSpace(above[len(above)-1]) != ""
			doc.sections = append(doc.sections, cur)
			continue
		}
		if cur == nil {
			if fence == "" && headingRE.MatchString(line) {
				titled = true
			}
			doc.head = append(doc.head, line)
			continue
		}
		cur.lines = append(cur.lines, line)
		if e, ok := parseEntry(line); ok {
			cur.entries = append(cur.entries, e)
		} else if strings.TrimSpace(line) != "" {
			cur.plain = false
		}
	}
	// A list of links that ends the head, before any section heading, is a
	// section without a heading: "# Title", a paragraph, then the list.
	k := len(doc.head)
	for i := len(doc.head) - 1; i >= 0; i-- {
		if strings.TrimSpace(doc.head[i]) == "" {
			continue
		}
		if _, ok := parseEntry(doc.head[i]); !ok {
			break
		}
		k = i
	}
	if k < len(doc.head) {
		s := &section{lines: doc.head[k:], plain: true}
		s.tightBefore = k > 0 && strings.TrimSpace(doc.head[k-1]) != ""
		for _, line := range s.lines {
			if e, ok := parseEntry(line); ok {
				s.entries = append(s.entries, e)
			}
		}
		doc.head = doc.head[:k]
		doc.sections = append([]*section{s}, doc.sections...)
	}
	for _, s := range doc.sections {
		s.tightHeading = s.heading != "" && len(s.lines) > 0 && strings.TrimSpace(s.lines[0]) != ""
		for _, line := range trimBlank(s.lines) {
			if strings.TrimSpace(line) == "" {
				s.loose = true
			}
		}
	}
	return doc
}

var (
	headingRE = regexp.MustCompile(`^ {0,3}#{1,6}(?:[ \t]|$)`)
	fenceRE   = regexp.MustCompile("^ {0,3}(?:```|~~~)")
)

// readHead returns the lines above the first section of the index at path.
func readHead(path string) []string {
	doc, err := parseIndex(path)
	if err != nil {
		return nil
	}
	return doc.head
}

// folderSummary is the description paragraph of the folder index at path,
// for the parent's index. A paragraph with a relative link or a wikilink is
// not used: the link is relative to the folder and would break one level up,
// and such a line is usually a stray entry, not a description.
func folderSummary(path string) string {
	p := introParagraph(readHead(path))
	if strings.Contains(p, "[[") {
		return ""
	}
	for _, m := range mdLinkRE.FindAllStringSubmatch(p, -1) {
		if !schemeRE.MatchString(m[1]) {
			return ""
		}
	}
	return p
}

var (
	mdLinkRE = regexp.MustCompile(`\]\(\s*<?([^)>\s]*)`)
	schemeRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

// introParagraph is the first paragraph under the heading: the folder's
// description, as one line. List items, and lines with nothing but markup
// (a left-over "* []() -"), are not a description: they would replace a
// good summary in the parent's index with noise.
func introParagraph(head []string) string {
	var para []string
	for _, line := range head {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "#"):
			continue
		case t == "", listItemRE.MatchString(t), !hasText(t):
			if len(para) > 0 {
				return strings.Join(para, " ")
			}
		default:
			para = append(para, t)
		}
	}
	return strings.Join(para, " ")
}

var listItemRE = regexp.MustCompile(`^(?:[-*+]|\d+[.)])(?:\s|$)`)

// hasText reports whether line has a letter or digit outside link targets,
// HTML comments and markup.
func hasText(line string) bool {
	line = linkTargetRE.ReplaceAllString(line, "]")
	line = commentRE.ReplaceAllString(line, "")
	for _, r := range line {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

var (
	linkTargetRE = regexp.MustCompile(`\]\([^)]*\)`)
	commentRE    = regexp.MustCompile(`<!--.*?-->`)
)

// pageDescription is the page's frontmatter description on one line.
func pageDescription(path string) string {
	c, err := concept.ParseLenient(path, filepath.Base(path))
	if err != nil {
		return ""
	}
	return strings.Join(strings.Fields(c.Frontmatter.Description), " ")
}

// fillDescription adds "description: desc" to the frontmatter of the page at
// path when it has frontmatter and no description yet. The line goes after
// title, else after type, else first.
func fillDescription(path, desc string, dryRun bool) (bool, error) {
	c, err := concept.ParseLenient(path, filepath.Base(path))
	if err != nil || c.FrontmatterErr != nil || c.Frontmatter.Description != "" {
		return false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	text := strings.TrimPrefix(string(raw), "\uFEFF")
	lines := strings.Split(text, "\n")
	// The closing delimiter of the frontmatter ParseLenient just accepted.
	closing := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			closing = i
			break
		}
	}
	if closing == -1 {
		return false, nil
	}
	y, err := yaml.Marshal(map[string]string{"description": desc})
	if err != nil {
		return false, err
	}
	newLine := strings.TrimRight(string(y), "\n")
	// After title, else after type, else first; a value that continues on
	// indented lines belongs to the key above.
	at := 0
	for _, key := range []string{"title:", "type:"} {
		for i := 1; i < closing; i++ {
			if strings.HasPrefix(lines[i], key) {
				at = i
				break
			}
		}
		if at > 0 {
			break
		}
	}
	for at+1 < closing && (strings.HasPrefix(lines[at+1], " ") || strings.HasPrefix(lines[at+1], "\t")) {
		at++
	}
	if dryRun {
		return true, nil
	}
	out := append([]string{}, lines[:at+1]...)
	out = append(out, newLine)
	out = append(out, lines[at+1:]...)
	return true, os.WriteFile(path, []byte(strings.Join(out, "\n")), 0644) //nolint:gosec // the bundle's own page
}

// isListedFolder reports whether dir belongs in its parent's folder list: it
// has an index.md, or a page (a non-reserved .md) somewhere inside.
func isListedFolder(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		n := strings.ToLower(d.Name())
		if strings.HasSuffix(n, ".md") && (n == "index.md" || !concept.ReservedNames[n]) && !strings.HasPrefix(n, ".") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// keepOrder returns names in the order the index listed them, so an order
// chosen by hand stays; names new to the index follow, in sorted order.
func keepOrder(sorted, old []string) []string {
	have := map[string]bool{}
	for _, n := range sorted {
		have[n] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, n := range old {
		if have[n] && !seen[n] {
			out = append(out, n)
			seen[n] = true
		}
	}
	for _, n := range sorted {
		if !seen[n] {
			out = append(out, n)
		}
	}
	return out
}

func sortNames(s []string) {
	sort.SliceStable(s, func(i, j int) bool {
		a, b := strings.ToLower(s[i]), strings.ToLower(s[j])
		if a != b {
			return a < b
		}
		return s[i] < s[j]
	})
}

func trimBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	return lines
}

// escape writes a file name as a link target: only the characters that would
// break a Markdown link are escaped, so Japanese names stay readable.
func escape(name string) string {
	r := strings.NewReplacer("%", "%25", " ", "%20", "(", "%28", ")", "%29")
	return r.Replace(name)
}

func unescape(target string) string {
	if u, err := url.PathUnescape(target); err == nil {
		return u
	}
	return target
}
