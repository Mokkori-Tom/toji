package pagereplace

import (
	"errors"
	"strings"
	"testing"

	"github.com/Mokkori-Tom/toji/internal/pageappend"
)

const page = "---\ntype: Note\n---\n\n# A\n\n## 関連\n\n- [x](./x.md)\n- 古い行\n\n## 記録\n\n- 古い行\n- 2026-11-23: y\n"

func TestReplace_Unique(t *testing.T) {
	out, lines, err := Replace(page, "- 2026-11-23: y", "- 2026-11-23: z", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "- 2026-11-23: z\n") || strings.Contains(out, ": y") || len(lines) != 1 || lines[0] != 15 {
		t.Errorf("out %q lines %v", out, lines)
	}
}

func TestReplace_AmbiguousNamesLines(t *testing.T) {
	_, _, err := Replace(page, "- 古い行", "x", "", false)
	var amb *AmbiguousError
	if !errors.As(err, &amb) || len(amb.Lines) != 2 || amb.Lines[0] != 10 || amb.Lines[1] != 14 {
		t.Fatalf("err %v", err)
	}
}

func TestReplace_SectionLimitsTheSearch(t *testing.T) {
	out, lines, err := Replace(page, "- 古い行", "- 新しい行", "記録", false)
	if err != nil {
		t.Fatal(err)
	}
	if lines[0] != 14 || !strings.Contains(out, "- [x](./x.md)\n- 古い行\n") || !strings.Contains(out, "## 記録\n\n- 新しい行\n") {
		t.Errorf("out %q", out)
	}
}

func TestReplace_All(t *testing.T) {
	out, lines, err := Replace(page, "古い行", "新しい行", "", true)
	if err != nil || len(lines) != 2 || strings.Contains(out, "古い行") {
		t.Fatalf("out %q lines %v err %v", out, lines, err)
	}
}

func TestReplace_DeleteWholeLinesLeavesNoGap(t *testing.T) {
	out, _, err := Replace(page, "- 古い行\n- 2026-11-23: y", "", "記録", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out, "## 記録\n\n") {
		t.Errorf("out %q", out)
	}
	// A part of a line is deleted alone, the line stays.
	out, _, _ = Replace(page, ": y", "", "", false)
	if !strings.HasSuffix(out, "- 2026-11-23\n") {
		t.Errorf("partial delete %q", out)
	}
}

func TestReplace_NotFound(t *testing.T) {
	_, _, err := Replace(page, "- [x](./x.md)", "y", "記録", false)
	var nf *NotFoundError
	if !errors.As(err, &nf) || nf.Elsewhere != 1 {
		t.Fatalf("outside the section: %v", err)
	}
	_, _, err = Replace(page, "-  2026-11-23:   y", "z", "", false)
	if !errors.As(err, &nf) || !nf.SpaceMatch {
		t.Fatalf("spacing: %v", err)
	}
	if _, _, err := Replace(page, "  ", "z", "", false); err == nil {
		t.Error("empty old text accepted")
	}
}

func TestReplace_MissingSection(t *testing.T) {
	_, _, err := Replace(page, "x", "y", "無い", false)
	var nf *pageappend.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err %v", err)
	}
}

func TestReplace_KeepsCRLF(t *testing.T) {
	in := strings.ReplaceAll(page, "\n", "\r\n")
	out, _, err := Replace(in, "- 2026-11-23: y\n", "- z\n", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "\r\n") != strings.Count(out, "\n") || !strings.Contains(out, "- z\r\n") {
		t.Errorf("out %q", out)
	}
}

func TestReplace_SectionIsLastAndHeadingIncluded(t *testing.T) {
	out, _, err := Replace(page, "## 記録", "## 記録（旧）", "記録", false)
	if err != nil || !strings.Contains(out, "## 記録（旧）\n") {
		t.Fatalf("out %q err %v", out, err)
	}
}
