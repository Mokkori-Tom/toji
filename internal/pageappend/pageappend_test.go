package pageappend

import (
	"errors"
	"testing"
)

func TestAppend_End(t *testing.T) {
	in := "---\ntype: Note\n---\n\n# T\n\nbody\n\n\n"
	out, line, err := Append(in, "\n- added\n", "")
	if err != nil {
		t.Fatal(err)
	}
	want := "---\ntype: Note\n---\n\n# T\n\nbody\n\n- added\n"
	if out != want {
		t.Errorf("got:\n%q\nwant:\n%q", out, want)
	}
	if line != 9 {
		t.Errorf("line = %d, want 9", line)
	}
}

func TestAppend_Section(t *testing.T) {
	in := "# T\n\n## 結論\n\n- a\n\n### 詳細\n\nx\n\n## 未検証\n\n- [ ] y\n"
	out, line, err := Append(in, "- b", "結論")
	if err != nil {
		t.Fatal(err)
	}
	// The section runs past its ### subsection to the next ##.
	want := "# T\n\n## 結論\n\n- a\n\n### 詳細\n\nx\n\n- b\n\n## 未検証\n\n- [ ] y\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
	if line != 11 {
		t.Errorf("line = %d, want 11", line)
	}

	// The last section: the block goes at the end of the page.
	out, _, err = Append(in, "- [ ] z", "## 未検証")
	if err != nil {
		t.Fatal(err)
	}
	if want := in + "- [ ] z\n"; out != want {
		t.Errorf("last section:\n%s\nwant:\n%s", out, want)
	}
}

func TestAppend_HeadingMatching(t *testing.T) {
	in := "# T\n\n```md\n## 関連\n```\n\n## Related  Pages\n\ntext\n"
	if _, _, err := Append(in, "x", "related pages"); err != nil {
		t.Errorf("case and spacing should not matter: %v", err)
	}
	var nf *NotFoundError
	if _, _, err := Append(in, "x", "関連"); !errors.As(err, &nf) {
		t.Errorf("a heading inside a code fence must not match, got %v", err)
	} else if len(nf.Headings) != 2 {
		t.Errorf("headings offered: %v", nf.Headings)
	}

	dup := "# T\n\n## メモ\n\na\n\n## メモ\n\nb\n"
	var amb *AmbiguousError
	if _, _, err := Append(dup, "x", "メモ"); !errors.As(err, &amb) || len(amb.Lines) != 2 {
		t.Errorf("duplicate heading: %v", err)
	}
}

func TestAppend_KeepsCRLFAndRejectsEmpty(t *testing.T) {
	out, _, err := Append("# T\r\n\r\nbody\r\n", "x", "")
	if err != nil {
		t.Fatal(err)
	}
	if out != "# T\r\n\r\nbody\r\n\r\nx\r\n" {
		t.Errorf("CRLF page: %q", out)
	}
	if _, _, err := Append("# T\n", " \n\n", ""); err == nil {
		t.Error("an empty body must be refused")
	}
}
