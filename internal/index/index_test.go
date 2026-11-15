package index

import (
	"os"
	"path/filepath"
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
