package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/concept"
)

func TestSearch_ByTag(t *testing.T) {
	b := testBundle(t, map[string]string{
		"tables/users.md":    "---\ntype: Table\ntitle: Users\ntags: [auth, users]\n---\n\nbody",
		"tables/orders.md":   "---\ntype: Table\ntitle: Orders\ntags: [sales, orders]\n---\n\nbody",
		"tables/sessions.md": "---\ntype: Table\ntitle: Sessions\ntags: [auth]\n---\n\nbody",
	})

	results := Search(b, Filters{Tag: "auth"})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	ids := resultIDs(results)
	if !contains(ids, "tables/users") || !contains(ids, "tables/sessions") {
		t.Errorf("expected tables/users and tables/sessions, got %v", ids)
	}
}

func TestSearch_ByType(t *testing.T) {
	b := testBundle(t, map[string]string{
		"tables/users.md":  "---\ntype: Table\ntitle: Users\n---\n\nbody",
		"datasets/ga4.md":  "---\ntype: Dataset\ntitle: GA4\n---\n\nbody",
		"tables/orders.md": "---\ntype: Table\ntitle: Orders\n---\n\nbody",
	})

	results := Search(b, Filters{Type: "Table"})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
}

func TestSearch_ByText(t *testing.T) {
	b := testBundle(t, map[string]string{
		"tables/users.md":    "---\ntype: Table\ntitle: Users\n---\n\n# Users\n\nOne row per authenticated user account.",
		"tables/orders.md":   "---\ntype: Table\ntitle: Orders\n---\n\n# Orders\n\nOne row per customer order.",
		"tables/products.md": "---\ntype: Table\ntitle: Products\n---\n\n# Products\n\nProduct catalog.",
	})

	results := Search(b, Filters{Text: "order"})
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].ID != "tables/orders" {
		t.Errorf("got %s, want tables/orders", results[0].ID)
	}
}

func TestSearch_TextMatchesTitleAndDescription(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: Revenue Report\ndescription: Monthly revenue breakdown\n---\n\nbody",
		"b.md": "---\ntype: T\ntitle: Logs\ndescription: System logs\n---\n\nbody",
	})

	results := Search(b, Filters{Text: "revenue"})
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (should match title)", len(results))
	}
}

func TestSearch_CombinedFilters(t *testing.T) {
	b := testBundle(t, map[string]string{
		"tables/users.md":   "---\ntype: Table\ntitle: Users\ntags: [auth]\n---\n\nuser accounts",
		"tables/orders.md":  "---\ntype: Table\ntitle: Orders\ntags: [auth]\n---\n\norder data",
		"datasets/users.md": "---\ntype: Dataset\ntitle: Users\ntags: [auth]\n---\n\nuser dataset",
	})

	results := Search(b, Filters{Type: "Table", Tag: "auth"})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (Table + auth tag)", len(results))
	}
}

func TestSearch_NoFiltersReturnsAll(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\n---\n\nbody",
		"b.md": "---\ntype: T\ntitle: B\n---\n\nbody",
	})

	results := Search(b, Filters{})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
}

func TestSearch_EmptyBundle(t *testing.T) {
	b := testBundle(t, nil)
	results := Search(b, Filters{Tag: "anything"})
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

func TestSearch_TextIsCaseInsensitive(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: Users\n---\n\nUser Accounts",
	})

	results := Search(b, Filters{Text: "USER"})
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (case insensitive)", len(results))
	}
}

// --- helpers ---

func resultIDs(results []*concept.Concept) []string {
	ids := make([]string, len(results))
	for i, r := range results {
		ids[i] = r.ID
	}
	return ids
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func testBundle(t *testing.T, files map[string]string) *bundle.Bundle {
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
	b, err := bundle.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

func TestFind_Meta(t *testing.T) {
	b := testBundle(t, map[string]string{
		"index.md":     "---\nokf_version: \"0.2\"\n---\n\n- [hw/](./hw/) - fan and gpu notes\n",
		"hw/index.md":  "# hw\n\n- [fan](./fan.md) - quiet fan setup\n- [gpu](./gpu.md) - gpu notes\n",
		"hw/fan.md":    "---\ntype: Note\ntitle: Fan\n---\n\nThe gpu runs hot.",
		"hw/gpu.md":    "---\ntype: Note\ntitle: GPU\ndescription: card choice\ntags: [hardware]\n---\n\nbody",
		"hw/orphan.md": "---\ntype: Note\ntitle: Orphan\n---\n\nquiet body",
	})

	// "gpu" is in fan's body only, so Meta skips it; the folder entry in the
	// root index names no page.
	hits := Find(b, Filters{Terms: []string{"gpu"}, Meta: true})
	if len(hits) != 1 || hits[0].Concept.ID != "hw/gpu" {
		t.Fatalf("got %v, want [hw/gpu]", hitIDs(hits))
	}
	if hits[0].Summary != "card choice" || hits[0].Lines != nil {
		t.Errorf("Summary %q, Lines %v; want the description and no lines", hits[0].Summary, hits[0].Lines)
	}

	// The index entry is searched and becomes the summary when there is no
	// description; orphan's body is not searched.
	hits = Find(b, Filters{Terms: []string{"quiet"}, Meta: true})
	if len(hits) != 1 || hits[0].Concept.ID != "hw/fan" || hits[0].Summary != "quiet fan setup" {
		t.Fatalf("got %v, want hw/fan with the index summary", hits)
	}

	if hits := Find(b, Filters{Terms: []string{"hardware"}, Meta: true}); len(hitIDs(hits)) != 1 {
		t.Errorf("tags: got %v, want [hw/gpu]", hitIDs(hits))
	}
}

func TestEntryText(t *testing.T) {
	cases := map[string]string{
		"- [A](./a.md) - one line":        "one line",
		"- [A](./a.md): one line":         "one line",
		"- [A](./a.md)":                   "",
		"| [A](a.md) | Note | one line |": "Note | one line",
	}
	for line, want := range cases {
		target := "./a.md"
		if strings.HasPrefix(line, "|") {
			target = "a.md"
		}
		if got := entryText(line, target); got != want {
			t.Errorf("entryText(%q) = %q, want %q", line, got, want)
		}
	}
}

func hitIDs(hits []Hit) []string {
	var ids []string
	for _, h := range hits {
		ids = append(ids, h.Concept.ID)
	}
	return ids
}
