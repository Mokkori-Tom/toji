package index

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

func read(t *testing.T, dir, p string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, p))
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

func TestGenerate_WritesIndexes(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"tables/users.md": "---\ntype: Table\ntitle: Users\ndescription: d\n---\n\nbody",
	})
	res, err := Generate(dir, Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, p := range []string{"index.md", "tables/index.md"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
	if len(res.Indexes) != 2 || res.Indexes[0].Status != "created" {
		t.Errorf("result: %+v", res.Indexes)
	}
	want := "# tables\n\n## Documents\n\n- [users](./users.md) - d\n"
	if got := read(t, dir, "tables/index.md"); got != want {
		t.Errorf("tables/index.md:\n%s\nwant:\n%s", got, want)
	}
	// Nothing written by hand yet: both folders need a description.
	for _, ir := range res.Indexes {
		if ir.HasDescription {
			t.Errorf("%s: HasDescription without a paragraph", ir.Dir)
		}
	}
}

// §12: a bundle-root index.md may declare okf_version; regeneration must
// preserve an existing declaration rather than dropping it.
func TestGenerate_PreservesRootOkfVersion(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"index.md":        "---\nokf_version: \"0.2\"\n---\n# Index\n\nstale content\n\n## Documents\n\n- [gone](./gone.md) - was here\n",
		"tables/users.md": "---\ntype: Table\ntitle: Users\ndescription: d\n---\n\nbody",
	})
	if _, err := Generate(dir, Options{}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	s := read(t, dir, "index.md")
	if !strings.HasPrefix(s, "---\n") || !strings.Contains(s, `okf_version: "0.2"`) {
		t.Errorf("root index.md lost its okf_version declaration:\n%s", s)
	}
	// The paragraph is the folder's description and stays; the stale entry goes.
	if !strings.Contains(s, "stale content") || strings.Contains(s, "gone") {
		t.Errorf("root index.md: paragraph must stay, stale entry must go:\n%s", s)
	}
}

// A root index with no prior declaration must not gain one implicitly (the
// bundle may target v0.1), and non-root indexes never carry frontmatter (§8).
func TestGenerate_NoImplicitOkfVersion(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"tables/users.md": "---\ntype: Table\ntitle: Users\ndescription: d\n---\n\nbody",
	})
	if _, err := Generate(dir, Options{}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.Contains(read(t, dir, "index.md"), "okf_version") {
		t.Errorf("root index.md gained an implicit okf_version")
	}
	if strings.HasPrefix(read(t, dir, "tables/index.md"), "---\n") {
		t.Errorf("non-root index.md must not carry frontmatter")
	}
}

// A hand-written vault index: Japanese headings, a folder paragraph, a guide
// section that is not a plain list, summaries written by hand.
func TestGenerate_KeepsHandwrittenParts(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"index.md":      "---\nokf_version: \"0.2\"\n---\n\n# ドキュメント一覧\n\n## 案内\n\n- [このバンドルについて](./README.md) - 分け方\n- [更新履歴](./log.md) - 記録\n\n## フォルダ\n\n- [hw/](./hw/) - 古い説明\n- [removed/](./removed/) - もう無い\n",
		"README.md":     "---\ntype: readme\ntitle: about\ndescription: from frontmatter\n---\n\n# about\n",
		"log.md":        "# log\n",
		"hw/index.md":   "# hw\n\n機器のメモ\n\n## 文書\n\n- [fan](./fan.md) - 手書きの要約\n- [gpu](./gpu.md) - 古い要約\n\n## ファイル\n\n- [pic.png](./pic.png) - 写真\n",
		"hw/fan.md":     "---\ntype: Note\ntitle: Fan\n---\n\n# Fan\n",
		"hw/gpu.md":     "---\ntype: Note\ntitle: GPU\ndescription: カード選び\n---\n\n# GPU\n",
		"hw/new (1).md": "---\ntype: Note\ntitle: New\n---\n\n# New\n",
		"hw/pic.png":    "x",
		"hw/.hidden.md": "x",
	})
	res, err := Generate(dir, Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	wantRoot := "---\nokf_version: \"0.2\"\n---\n# ドキュメント一覧\n\n## 案内\n\n- [このバンドルについて](./README.md) - 分け方\n- [更新履歴](./log.md) - 記録\n\n## フォルダ\n\n- [hw/](./hw/) - 機器のメモ\n"
	if got := read(t, dir, "index.md"); got != wantRoot {
		t.Errorf("root:\n%s\nwant:\n%s", got, wantRoot)
	}
	wantHW := "# hw\n\n機器のメモ\n\n## 文書\n\n- [fan](./fan.md) - 手書きの要約\n- [gpu](./gpu.md) - カード選び\n- [new (1)](./new%20%281%29.md)\n\n## ファイル\n\n- [pic.png](./pic.png) - 写真\n"
	if got := read(t, dir, "hw/index.md"); got != wantHW {
		t.Errorf("hw:\n%s\nwant:\n%s", got, wantHW)
	}
	for _, ir := range res.Indexes {
		switch ir.Dir {
		case ".":
			if ir.HasDescription || ir.Status != "updated" {
				t.Errorf("root: %+v", ir)
			}
		case "hw":
			if !ir.HasDescription || ir.Status != "updated" || ir.Entries != 4 {
				t.Errorf("hw: %+v", ir)
			}
		}
	}
	// A second run changes nothing.
	res, _ = Generate(dir, Options{})
	for _, ir := range res.Indexes {
		if ir.Status != "unchanged" {
			t.Errorf("second run: %s %s", ir.Dir, ir.Status)
		}
	}
}

func TestGenerate_DryRunAndFill(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"hw/index.md": "# hw\n\n## 文書\n\n- [fan](./fan.md) - 静かなファン\n- [gpu](./gpu.md) - 途中で切れた…\n- [has](./has.md) - 無視される\n",
		"hw/fan.md":   "---\ntype: Note\ntitle: Fan\ntags:\n  - a\n---\n\n# Fan\n",
		"hw/gpu.md":   "---\ntype: Note\ntitle: GPU\n---\n\n# GPU\n",
		"hw/has.md":   "---\ntype: Note\ndescription: already\n---\n\n# has\n",
	})
	before := read(t, dir, "hw/fan.md")
	res, err := Generate(dir, Options{DryRun: true, FillDescriptions: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res.Filled) != 1 || res.Filled[0] != "hw/fan.md" {
		t.Errorf("Filled = %v, want [hw/fan.md]", res.Filled)
	}
	if read(t, dir, "hw/fan.md") != before {
		t.Errorf("dry run wrote the page")
	}
	if _, err := os.Stat(filepath.Join(dir, "index.md")); err == nil {
		t.Errorf("dry run wrote the root index")
	}

	if _, err := Generate(dir, Options{FillDescriptions: true}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	want := "---\ntype: Note\ntitle: Fan\ndescription: 静かなファン\ntags:\n  - a\n---\n\n# Fan\n"
	if got := read(t, dir, "hw/fan.md"); got != want {
		t.Errorf("fan.md:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(read(t, dir, "hw/gpu.md"), "description") {
		t.Errorf("truncated summary was copied into gpu.md")
	}
}

func TestEntryParse(t *testing.T) {
	cases := map[string]entry{
		"- [a](./a.md) - one":            {text: "a", target: "./a.md", desc: "one"},
		"- [a](./a.md): one":             {text: "a", target: "./a.md", desc: "one"},
		"- [a](./a.md)":                  {text: "a", target: "./a.md"},
		"- [x/](./x/) - フォルダ":            {text: "x/", target: "./x/", desc: "フォルダ"},
		"- [b](./b%20c%28d%29.md) - two": {text: "b", target: "./b%20c%28d%29.md", desc: "two"},
	}
	for line, want := range cases {
		got, ok := parseEntry(line)
		if !ok || got != want {
			t.Errorf("parseEntry(%q) = %+v, %v; want %+v", line, got, ok, want)
		}
	}
	if e, _ := parseEntry("- [b](./b%20c%28d%29.md)"); e.name() != "b c(d).md" {
		t.Errorf("name() = %q", e.name())
	}
}

// A folder whose index.md head holds only a left-over list line must not
// replace the summary its parent's index already has.
func TestGenerate_MarkupIsNotAFolderDescription(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"index.md":     "# Index\n\n## Folders\n\n- [gtd/](gtd/) - GTDとOKFを融合させる\n",
		"gtd/index.md": "# \n\n* []() -",
		"gtd/a.md":     "---\ntype: Note\ndescription: d\n---\n\nbody",
	})
	res, err := Generate(dir, Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got := read(t, dir, "index.md"); !strings.Contains(got, "- [gtd/](gtd/) - GTDとOKFを融合させる\n") {
		t.Errorf("parent summary lost or link rewritten:\n%s", got)
	}
	for _, ir := range res.Indexes {
		if ir.Dir == "gtd" && ir.HasDescription {
			t.Error("gtd: a bare list line counted as a description")
		}
	}
	for _, c := range []struct {
		head []string
		want string
	}{
		{[]string{"# x", "", "- item", "", "Real text."}, "Real text."},
		{[]string{"# x", "", "Para one", "continues.", "- list"}, "Para one continues."},
		{[]string{"# x", "", "<!-- note -->", "", "[link](./a.md) text"}, "[link](./a.md) text"},
		{[]string{"# x", "", "1. first"}, ""},
	} {
		if got := introParagraph(c.head); got != c.want {
			t.Errorf("introParagraph(%q) = %q, want %q", c.head, got, c.want)
		}
	}
}

func TestGenerate_DirsLimitsTheRun(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"index.md":   "# Index\n\nold root\n",
		"a/index.md": "# a\n\nfolder a\n",
		"a/p.md":     "---\ntype: Note\ndescription: p\n---\n\nbody",
		"a/b/q.md":   "---\ntype: Note\ndescription: q\n---\n\nbody",
		"c/r.md":     "---\ntype: Note\ndescription: r\n---\n\nbody",
	})
	res, err := Generate(dir, Options{Dirs: []string{"a/"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res.Indexes) != 1 || res.Indexes[0].Dir != "a" || res.Indexes[0].Status != "updated" {
		t.Fatalf("result: %+v", res.Indexes)
	}
	if got := read(t, dir, "index.md"); got != "# Index\n\nold root\n" {
		t.Errorf("root index touched:\n%s", got)
	}
	for _, p := range []string{"a/b/index.md", "c/index.md"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			t.Errorf("%s created outside --dir", p)
		}
	}
	for _, bad := range []string{"../x", "missing", "a/p.md"} {
		var de *DirError
		if _, err := Generate(dir, Options{Dirs: []string{bad}}); err == nil || !errorsAs(err, &de) {
			t.Errorf("Dirs %q: err = %v, want DirError", bad, err)
		}
	}
}

func TestGenerate_DiffAndFinalNewline(t *testing.T) {
	idx := "# Index\n\nroot\n\n## Documents\n\n- [a](./a.md) - a"
	dir := writeFiles(t, map[string]string{
		"index.md": idx, // no final newline
		"a.md":     "---\ntype: Note\ndescription: a\n---\n\nbody",
	})
	res, err := Generate(dir, Options{Diff: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if ir := res.Indexes[0]; ir.Status != "unchanged" || ir.Diff != "" {
		t.Errorf("only the final newline differs: %+v", ir)
	}
	if got := read(t, dir, "index.md"); got != idx {
		t.Errorf("file rewritten: %q", got)
	}

	writeFiles2(t, dir, map[string]string{"b.md": "---\ntype: Note\ndescription: b\n---\n\nbody"})
	res, err = Generate(dir, Options{Diff: true, DryRun: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	want := "--- a/index.md\n+++ b/index.md\n@@ -5,3 +5,4 @@\n ## Documents\n \n - [a](./a.md) - a\n+- [b](./b.md) - b\n"
	if got := res.Indexes[0].Diff; got != want {
		t.Errorf("diff:\n%s\nwant:\n%s", got, want)
	}
	if got := read(t, dir, "index.md"); got != idx {
		t.Error("dry run wrote the index")
	}
}

func TestUnifiedDiff_SeparateHunks(t *testing.T) {
	var a, b []string
	for i := 1; i <= 20; i++ {
		a = append(a, "l"+strconv.Itoa(i))
	}
	b = append(b, a...)
	b[1] = "X"
	b[17] = "Y"
	got := unifiedDiff(strings.Join(a, "\n")+"\n", strings.Join(b, "\n")+"\n", "f")
	if n := strings.Count(got, "@@ -"); n != 2 {
		t.Errorf("want 2 hunks, got %d:\n%s", n, got)
	}
	if !strings.Contains(got, "@@ -1,5 +1,5 @@\n l1\n-l2\n+X\n") || !strings.Contains(got, "@@ -15,6 +15,6 @@\n") {
		t.Errorf("hunks:\n%s", got)
	}
}

func writeFiles2(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(c), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func errorsAs(err error, target any) bool { return errors.As(err, target) }
