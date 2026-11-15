package toc

import (
	"strings"
	"testing"
)

const page = `---
type: Note
title: T
---

# タイトル

前書き。

## 症状 <!-- omit in toc -->

## 原因と対処

### GPU の温度

text

### GPU の温度

## 目次

` + "```" + `
# not a heading
` + "```" + `

#### 深すぎる

## [リンク](./x.md) 付き
`

func TestHeadings(t *testing.T) {
	hs := Headings(page, Options{})
	want := []Heading{
		{Level: 2, Text: "原因と対処", Slug: "原因と対処", Line: 12},
		{Level: 3, Text: "GPU の温度", Slug: "gpu-の温度", Line: 14},
		{Level: 3, Text: "GPU の温度", Slug: "gpu-の温度-1", Line: 18},
		{Level: 2, Text: "リンク 付き", Slug: "リンク-付き", Line: 28},
	}
	if len(hs) != len(want) {
		t.Fatalf("got %+v\nwant %+v", hs, want)
	}
	for i := range want {
		if hs[i] != want[i] {
			t.Errorf("heading %d: got %+v, want %+v", i, hs[i], want[i])
		}
	}
	if hs := Headings(page, Options{Depth: 4}); len(hs) != 5 || hs[3].Text != "深すぎる" {
		t.Errorf("depth 4: %+v", hs)
	}
}

func TestHeadings_SeveralH1AreSections(t *testing.T) {
	hs := Headings("# A\n\n## a1\n\n# B\n", Options{})
	if len(hs) != 3 || hs[0].Level != 1 || hs[2].Text != "B" {
		t.Errorf("got %+v", hs)
	}
}

func TestApplyInsertUpdateRemove(t *testing.T) {
	text := "---\ntype: Note\n---\n\n# T\n\nintro\n\n## A\n\n## B\n"
	list := Render(Headings(text, Options{}))
	if list != "- [A](#a)\n- [B](#b)\n" {
		t.Fatalf("list %q", list)
	}
	got, status := Apply(text, list)
	want := "---\ntype: Note\n---\n\n# T\n\n<!-- toc -->\n- [A](#a)\n- [B](#b)\n<!-- /toc -->\n\nintro\n\n## A\n\n## B\n"
	if status != "inserted" || got != want {
		t.Errorf("insert: %s\n%q\nwant\n%q", status, got, want)
	}

	if _, status := Apply(got, list); status != "unchanged" {
		t.Errorf("second apply: %s", status)
	}

	got2, status := Apply(got, "- [A](#a)\n")
	if status != "updated" || !strings.Contains(got2, "<!-- toc -->\n- [A](#a)\n<!-- /toc -->") || strings.Contains(got2, "#b") {
		t.Errorf("update: %s\n%q", status, got2)
	}

	back, status := Remove(got)
	if status != "removed" || back != text {
		t.Errorf("remove: %s\n%q\nwant\n%q", status, back, text)
	}
	if _, status := Remove(text); status != "unchanged" {
		t.Errorf("remove without markers: %s", status)
	}
}

func TestApply_NoTitle(t *testing.T) {
	text := "## A\n\ntext\n"
	got, status := Apply(text, "- [A](#a)\n")
	want := "<!-- toc -->\n- [A](#a)\n<!-- /toc -->\n\n## A\n\ntext\n"
	if status != "inserted" || got != want {
		t.Errorf("%s\n%q\nwant\n%q", status, got, want)
	}
	if _, status := Apply("just text\n", ""); status != "unchanged" {
		t.Errorf("empty list without markers must not insert markers")
	}
}

func TestApply_MarkdownAllInOneMarkers(t *testing.T) {
	text := "# T\n\n<!-- TOC -->\n- old\n<!-- /TOC -->\n\n## A\n"
	got, status := Apply(text, "- [A](#a)\n")
	if status != "updated" || !strings.Contains(got, "<!-- toc -->\n- [A](#a)\n<!-- /toc -->") || strings.Contains(got, "old") {
		t.Errorf("%s\n%q", status, got)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Hello, World!":    "hello-world",
		"GPU の温度":          "gpu-の温度",
		"1. REPL の活用 (基礎)": "1-repl-の活用-基礎",
		"a_b-c":            "a_b-c",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}
