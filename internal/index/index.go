// Package index writes index.md files for the directories of a bundle. Per
// OKF spec §6, index.md is a directory listing for progressive disclosure:
// agents and humans navigate one level at a time.
//
// An index is part written by hand and part derived, and Generate keeps the
// two apart. Hand-written: the heading, the paragraph under it (what the
// folder is for), and any section that is not a plain list of links. Derived
// and rebuilt on every run: the list of folders, the list of pages, and the
// list of other files, each entry "- [name](./target) - summary". A page's
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
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Mokkori-Tom/toji/internal/concept"
)

// Options controls Generate.
type Options struct {
	DryRun           bool // report what would change without writing
	FillDescriptions bool // give pages without a frontmatter description the summary their index carries
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
}

// Generate updates index.md in every directory of root that has pages, folders
// of pages, or an index.md already, and returns what changed.
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

// role is what a list section holds.
type role int

const (
	roleOther role = iota // kept as written
	roleFolder
	roleDoc
	roleFile
)

// names are the headings of the three list sections. Taken from the indexes
// the bundle already has, so a bundle that says "## フォルダ" keeps saying it.
type names map[role]string

var defaultNames = names{roleFolder: "Folders", roleDoc: "Documents", roleFile: "Files"}

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
			if r := s.uniformRole(filepath.Dir(path)); r != roleOther {
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
	case text == oldText:
		ir.Status = "unchanged"
	default:
		ir.Status = "updated"
	}
	if ir.Status != "unchanged" && !g.opt.DryRun {
		if err := os.WriteFile(indexPath, []byte(text), 0644); err != nil { //nolint:gosec // the bundle's own index.md
			return err
		}
	}
	res.Indexes = append(res.Indexes, ir)

	if g.opt.FillDescriptions && old != nil {
		for _, s := range old.sections {
			if s.uniformRole(dir) != roleDoc {
				continue
			}
			for _, e := range s.entries {
				if e.desc == "" || strings.HasSuffix(e.desc, "…") || strings.HasSuffix(e.desc, "...") {
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
		if len(old.head) > 0 {
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
	pinned := map[string]bool{}
	done := map[role]bool{}
	oldOrder := map[role][]string{}
	if old != nil {
		for _, s := range old.sections {
			r := s.uniformRole(dir)
			for _, e := range s.entries {
				if e.desc != "" {
					oldDesc[e.name()] = e.desc
				}
				if r == roleOther {
					pinned[e.name()] = true
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
	build := func(r role) []entry {
		var out []entry
		switch r {
		case roleFolder:
			for _, f := range keepOrder(l.folders, oldOrder[r]) {
				if pinned[f] {
					continue
				}
				d := desc(f, introParagraph(readHead(filepath.Join(dir, f, "index.md"))))
				out = append(out, entry{text: f + "/", target: "./" + escape(f) + "/", desc: d})
			}
		case roleDoc:
			for _, p := range keepOrder(l.pages, oldOrder[r]) {
				if pinned[p] {
					continue
				}
				d := desc(p, pageDescription(filepath.Join(dir, p)))
				out = append(out, entry{text: strings.TrimSuffix(p, ".md"), target: "./" + escape(p), desc: d})
			}
		case roleFile:
			for _, f := range keepOrder(l.files, oldOrder[r]) {
				if pinned[f] {
					continue
				}
				out = append(out, entry{text: f, target: "./" + escape(f), desc: oldDesc[f]})
			}
		}
		return out
	}
	writeSection := func(heading string, es []entry) {
		if len(es) == 0 {
			return
		}
		sb.WriteString("\n" + heading + "\n\n")
		for _, e := range es {
			sb.WriteString(e.String() + "\n")
		}
		ir.Entries += len(es)
	}

	// Sections in their old order: list sections rebuilt, others verbatim.
	if old != nil {
		for _, s := range old.sections {
			r := s.uniformRole(dir)
			if r == roleOther || done[r] {
				sb.WriteString("\n" + s.heading + "\n")
				if lines := trimBlank(s.lines); len(lines) > 0 {
					sb.WriteString("\n")
					for _, line := range lines {
						sb.WriteString(line + "\n")
					}
				}
				continue
			}
			done[r] = true
			writeSection(s.heading, build(r))
		}
	}
	for _, r := range []role{roleFolder, roleDoc, roleFile} {
		if !done[r] {
			writeSection("## "+g.names[r], build(r))
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
	heading string   // the "## ..." line
	lines   []string // every line under it, as written
	entries []entry  // the link entries among them
	plain   bool     // every non-blank line is an entry
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
	text   string
	target string
	desc   string
}

var entryRE = regexp.MustCompile(`^- \[([^\]]*)\]\(([^)\s]+)\)(?:\s*[-–—:：]\s*(.*))?$`)

func parseEntry(line string) (entry, bool) {
	m := entryRE.FindStringSubmatch(line)
	if m == nil {
		return entry{}, false
	}
	return entry{text: m[1], target: m[2], desc: strings.TrimSpace(m[3])}, true
}

func (e entry) String() string {
	s := "- [" + e.text + "](" + e.target + ")"
	if e.desc != "" {
		s += " - " + e.desc
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
	doc := &indexDoc{}
	if front, body, ok := concept.SplitFrontmatter(text); ok {
		doc.front = "---\n" + front + "\n---\n"
		text = body
	}
	var cur *section
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "## ") {
			cur = &section{heading: line, plain: true}
			doc.sections = append(doc.sections, cur)
			continue
		}
		if cur == nil {
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
	return doc
}

// readHead returns the lines above the first section of the index at path.
func readHead(path string) []string {
	doc, err := parseIndex(path)
	if err != nil {
		return nil
	}
	return doc.head
}

// introParagraph is the first paragraph under the heading: the folder's
// description, as one line.
func introParagraph(head []string) string {
	var para []string
	for _, line := range head {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "#"):
			continue
		case t == "":
			if len(para) > 0 {
				return strings.Join(para, " ")
			}
		default:
			para = append(para, t)
		}
	}
	return strings.Join(para, " ")
}

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
