# toji

**toji** (綴じ, "binding") is a command-line tool for a folder of Markdown notes: write a page, find it again in Japanese or English, read it in the terminal, and let an AI agent do the same through JSON. The notes are an [Open Knowledge Format (OKF)](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md) bundle - plain `.md` files with YAML frontmatter - so they stay readable by anything, and an existing Obsidian vault works as it is.

It began as a fork of [okfcli/okf](https://github.com/okfcli/okf), a validator and inspector for OKF bundles built to be driven by AI agents, and keeps all of it: `validate --strict` checks a bundle as OKF and nothing more lenient, every command prints JSON when piped, and `toji schema` describes the whole CLI for an agent. What toji adds is everyday use by a person:

日本語のはじめかた: [docs/GETTING_STARTED.ja.md](docs/GETTING_STARTED.ja.md)

- **Plain-text output on a terminal.** When stdout is a terminal, `list`, `search`, `show`, `backlinks`, `validate`, `lint` and `new` print readable text; piped or agent-driven runs still get the same JSON. Force a mode with `--json` / `--human` or `TOJI_OUTPUT=json|human`. `NO_COLOR` disables colors.
- **`toji new [bundle] <concept-id>`** writes a page with OKF frontmatter (`type`, `title`, empty `description`/`tags`, `generated.by/at`) and opens it in `$VISUAL`/`$EDITOR`. It never overwrites an existing page. Flags: `--title`, `--type` (default `Note`), `--tag a,b`, `--description`, `--status`, `--body <text>` or `--body -` (stdin; writes the body as given, so an agent creates a finished page in one call; a frontmatter block at its top is merged in, flags winning, so any extra keys such as `created` or `sources` can be set), `--template <name>` (from `$TOJI_TEMPLATES` or the bundle's `Templates/`; Obsidian-style `{{title}}`, `{{date}}`, `{{time}}`, `{{date:YYYY-MM-DD}}` plus offsets like `{{date+1d:...}}`, relative to the page's date when its title is one), `--no-edit`. Every command accepts `-h`/`--help`. The author is `$TOJI_AUTHOR`, else `human:$USER`.
- **Search that works for Japanese.** `toji search [bundle] 電力 上限` takes terms directly (AND; an argument is also split on whitespace, including the full-width space, while `--text` stays one phrase), matches substrings, and ignores case, full-width/half-width ASCII and katakana/hiragana (`ふぁん` finds `ファン`). Results are ranked (title matches first) and include matching lines with file line numbers (`matches`, `match_count` in JSON). `--text`, `--tag` and `--type` still work. `--meta` matches only the title, description, tags and the `index.md` line that links to a page, skipping the body, and returns each page's one-line `description` instead of `matches`: fewer, shorter results for an agent's first look.
- **Rendered pages.** On a terminal, `toji show` draws the page: headings without `#`, bullets, task boxes, Obsidian callouts, code blocks, tables aligned for Japanese text (falling back to one `header: value` block per row when too wide), and inline bold/italic/code/links with markup hidden. Wrapping follows Japanese line-breaking rules. Long pages go through `$TOJI_PAGER`, `$PAGER` or `less`. `--raw` prints the markdown as written, `--no-pager` skips the pager. In less, `v` opens the page's file in the editor; `toji edit <page>` does the same from the shell. No new dependencies.
- **`toji show` takes a file path.** `toji show notes/gpu.md` reads that file directly, without loading the rest of the bundle, so it works from anywhere and even on plain markdown without frontmatter.
- **`toji index` keeps what you wrote.** Each folder's `index.md` is part prose, part lists. The heading, the paragraph under it (what the folder is for) and any section that is not a plain list of links stay as written; the lists of folders, pages and other files are rebuilt as `- [name](./target) - summary`. A page's summary is its frontmatter `description`, a folder's is the paragraph at the top of its own `index.md`, and where neither exists the summary already in the index is kept. Section headings follow the ones the bundle uses (`## フォルダ` stays `## フォルダ`). `--dry-run` reports instead of writing; `missing_description` lists folders whose paragraph is still to be written; `--fill-descriptions` copies index summaries into pages that have no `description`.
- **`toji toc` writes a table of contents.** Like the Markdown All in One extension for VS Code: the page's headings become a list of links between `<!-- toc -->` and `<!-- /toc -->`, placed under the title the first time and rebuilt in place after that. One `#` heading is the title and is left out; headings up to `--depth` (default 3), outside code fences and not marked `<!-- omit in toc -->`, are listed. Fragments are GitHub style (lower case, spaces to hyphens, letters of any script kept), the same ids [okf-export](https://github.com/Mokkori-Tom/okf-export) gives headings. `--remove` takes it out again.
- **Plain notes are welcome.** A `.md` without frontmatter, or whose frontmatter has no `type` (an Obsidian vault, say), loads and searches like any concept; its title falls back to the first `# ` heading, then the file name. `validate` reports each such note with a single warning (`okf/frontmatter/missing` or `okf/frontmatter/type-required`) and skips the field checks. Adding `type` turns a note into an OKF concept and turns the full checks on. Frontmatter that is present but unparseable is an error (`okf/frontmatter/invalid`).
- **One bad file never hides the rest.** Unreadable files and broken frontmatter in `index.md`/`log.md` are skipped with a warning on stderr and reported by `validate` (`okf/bundle/unreadable`, `okf/frontmatter/invalid`). A bundle root that is a symlink is followed. `validate` fails on a bundle with no concepts (`okf/bundle/empty`), which is almost always a wrong path.
- **Shell completion.** `toji completion zsh >/dev/null 2>&1 && source <(toji completion zsh)` (or `bash`; the guard skips older toji builds that lack the command) completes commands, flags, page IDs for `show`/`backlinks`, folders for `new`, and values for `--tag`/`--type`, read live from the bundle.
- **Wikilinks count as links.** `[[page]]`, `[[page|alias]]`, `[[page#heading]]`, `[[folder/page]]` and `![[page]]` embeds are followed by `backlinks` and `graph`. They are not part of OKF, so `validate` does not check them: a link to a page not written yet is ordinary in a vault, and only OKF `[text](path.md)` links are reported as broken. They resolve like Obsidian: same folder, then the path from the root, then by page name anywhere (case-insensitive; same folder, then shortest path, wins). `[[/page]]` pins the root; links inside code and to attachments are ignored.
- **Results capped for agents.** `list`, `search` and `graph` return the first 50 results in JSON (`--limit N`, `0` for all); `count` stays the total and `truncated` says whether some were left out, so one call on a large bundle does not fill an agent's context. Plain-text output is not capped.
- **A Claude Code skill.** [docs/claude-code/toji/SKILL.md](docs/claude-code/toji/SKILL.md) tells Claude Code when to reach for `toji` and how. Install it with `ln -s "$PWD/docs/claude-code/toji" ~/.claude/skills/toji`.
- **Default bundle.** The bundle argument may be left out; `$TOJI_BUNDLE` is used, else the current directory.

## Install

```bash
go install github.com/Mokkori-Tom/toji/cmd/toji@latest
# or from a clone:
git clone https://github.com/Mokkori-Tom/toji.git && cd toji && make build   # ./bin/toji
# Termux: go build -o $PREFIX/bin/toji ./cmd/toji, or cross-build with
# CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build ./cmd/toji
```

Settings are read from `TOJI_BUNDLE`, `TOJI_AUTHOR`, `TOJI_TEMPLATES`, `TOJI_PAGER` and `TOJI_OUTPUT`; the older `OKF_*` names still work.

## Why a Go CLI for OKF?

Google's reference OKF implementation is Python + Gemini + BigQuery - vendor-locked to Google's cloud. okfcli/okf, and so toji, is the vendor-neutral alternative: a single Go binary that works anywhere, speaks JSON natively, and is designed to be driven by any AI agent on any provider.

**Agentic-first** means an AI agent can discover, understand, and drive the entire CLI without reading documentation or scraping text output. Three mechanisms make this work:

1. **`toji schema`** - emits a complete machine-readable description of every command: name, description, flags, arguments, output format, exit codes. One call and the agent knows the full CLI surface.

2. **JSON by default** - every command outputs structured JSON on stdout. No `--json` flag, no screen-scraping. Diagnostics go to stderr.

3. **Structured error envelopes** - all errors emit `{"error": {"kind":..., "code":..., "reason":..., "message":...}}` on stdout with a stable exit code. An agent can branch on the `kind` field to decide what to do next.

## Quick start

```bash
# Create a bundle, add concepts, validate
toji init ./my-bundle
# ... write .md files into tables/, datasets/, playbooks/ ...
toji validate ./my-bundle
toji index ./my-bundle
toji graph ./my-bundle
```

## Use cases

### 1. AI-driven documentation pipeline

An AI agent creates a bundle, writes concept documents from a database schema or API spec, validates them, and generates navigation - all autonomously.

```bash
toji init ./bundles/mydb                              # start from scratch
# AI writes tables/users.md, tables/orders.md, datasets/ga4.md, ...
toji validate ./bundles/mydb                           # catch spec violations
toji lint ./bundles/mydb                               # flag missing recommended fields
toji index ./bundles/mydb                              # generate index.md files
toji graph ./bundles/mydb                              # verify cross-link structure
```

The CLI is the quality gate. The AI reads validation findings as JSON, fixes each one, re-validates. No human in the loop unless something needs a judgment call.

### 2. Knowledge catalog audit

A company inherits a large OKF bundle from a migration or acquisition. An AI agent inspects it for health and quality without reading a single file:

```bash
toji list ./bundles/catalog                            # what's in here?
toji lint ./bundles/catalog                            # what's missing recommended fields?
toji graph ./bundles/catalog                           # find orphan concepts (isolated nodes)
toji search ./bundles/catalog --type Table             # filter by type
toji search ./bundles/catalog --text "deprecated"      # find stale concepts
toji backlinks ./bundles/catalog tables/users           # who depends on this concept?
toji show ./bundles/catalog tables/users               # read the full concept content
```

The agent produces a health report: orphans to fix, missing metadata to add, broken links to repair, dependency hotspots to document.

### 3. CI quality gate

Every PR that touches an OKF bundle runs `toji validate`. If it exits non-zero, the AI reads the findings and either fixes or comments on the PR:

```yaml
# .github/workflows/toji-check.yml
- run: toji validate ./bundles/ga4
# exit 1 -> parse findings -> fix or comment -> re-validate
```

The JSON error envelope makes this trivial to automate:

```json
{
  "error": {
    "kind": "validation",
    "code": 400,
    "reason": "validationError",
    "message": "load bundle ./bundles/ga4: parse notes/README.md: no YAML frontmatter block found"
  }
}
```

Broken cross-links are reported as warnings, never errors: OKF §6.1 says consumers must tolerate them because a link may point at not-yet-written knowledge, so they do not flip `valid` or the exit code.

Every finding also carries a stable rule ID (`okf/<family>/<check>`, e.g. `okf/links/broken`, `okf/frontmatter/type-required`), so automation can suppress or route specific rules instead of matching message text.

#### CI code scanning

`validate` can emit [SARIF 2.1.0](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html) instead of JSON, which GitHub ingests natively: findings show up as annotations on pull requests and in the repository's Security tab. `--exit-zero` reports findings without failing the step, so the upload always runs:

```yaml
# .github/workflows/toji-scan.yml
- run: toji validate --format sarif --exit-zero ./bundles/ga4 > toji.sarif
- uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: toji.sarif
```

### 4. Onboarding assistant

A new engineer joins a team. An AI agent walks them through the team's knowledge map:

```bash
toji list ./bundles/team                               # inventory
toji graph ./bundles/team                              # structure overview
toji show ./bundles/team tables/users                  # read a key concept
toji backlinks ./bundles/team tables/users             # what depends on it?
toji search ./bundles/team --tag auth                  # find related concepts
```

Progressive disclosure (index.md) lets the agent navigate level by level instead of loading the entire bundle.

## Commands

| Command | Description |
|---------|-------------|
| `toji schema [command]` | Print machine-readable CLI metadata as JSON |
| `toji init <bundle>` | Create a new empty OKF bundle |
| `toji validate <bundle> [--format json\|sarif] [--exit-zero]` | Validate a bundle against the OKF spec (exit 1 on errors) |
| `toji lint <bundle> [--format json\|sarif]` | Check recommended fields and style (warnings only) |
| `toji index [bundle] [--dry-run] [--fill-descriptions]` | Update the link lists in every index.md, keeping what was written by hand |
| `toji list <bundle> [--limit N]` | List concepts with ID, type, title, status, trust tier |
| `toji show [bundle] <concept-id \| file.md>` | Show a single concept's full content |
| `toji search [bundle] [terms...] [--tag] [--type] [--text] [--meta] [--limit N]` | Search concepts by tag, type, or text (`--meta`: title, description, tags and index entries only) |
| `toji new [bundle] <concept-id> [--title] [--type] [--tag] [--no-edit]` | Create a concept page and open it in $EDITOR |
| `toji toc [bundle] <concept-id \| file.md> [--depth N] [--remove] [--dry-run]` | Write a table of contents into a page, between `<!-- toc -->` markers |
| `toji backlinks <bundle> <concept-id>` | List concepts that link to a given concept |
| `toji graph <bundle> [--limit N]` | Print cross-link graph with nodes, edges, and stats |
| `toji version` | Print version |

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | validation error (spec violation, non-concept markdown file, concept not found) |
| 2 | filesystem or I/O error |
| 3 | internal error (unexpected) |
| 4 | usage error (missing args, unknown command) |

## What is OKF?

OKF is an open format from Google for representing knowledge - the metadata, context, and curated insight that surrounds data and systems. A bundle is a directory of markdown files with YAML frontmatter:

```
my-bundle/
├── index.md                  # directory listing (auto-generated)
├── tables/
│   ├── index.md
│   ├── users.md              # a concept
│   └── orders.md             # a concept
├── datasets/
│   └── ga4.md                # a concept
└── playbooks/
    └── freshness.md          # a concept
```

Each concept has YAML frontmatter and a markdown body with cross-links:

```markdown
---
type: Table
title: Customer Orders
description: One row per completed customer order.
tags: [sales, orders, revenue]
---

# Schema

| Column | Type | Description |
|--------|------|-------------|
| order_id | UUID | Primary key |
| user_id | UUID | FK to [Users](/tables/users.md) |

# Joins

Joined with [users](/tables/users.md) on user_id.
```

The format is intentionally minimal: no schema registry, no central authority, no required tooling. If you can `cat` a file, you can read OKF; if you can `git clone` a repo, you can ship it.

## OKF v0.2 support

`toji` implements [OKF v0.2](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md), which makes provenance, trust, lifecycle, and attestation first-class:

- **Provenance (§5.1)** - the `sources` family with per-source credibility signals (`author`, `usage_count`, `last_modified`) and `usage_window`; footnote labels are validated as join keys into `sources[].id` for per-claim attribution.
- **Trust (§5.2–5.3)** - `generated` and `verified` are parsed (a bare `verified` mapping is treated as a one-element list, as the spec requires) and every concept gets a derived trust tier: `unverified`, `machine-confirmed`, or `human-reviewed`.
- **Lifecycle (§5.4–5.5)** - `status` (draft/stable/deprecated, default stable) and `stale_after` (an RFC 3339 instant such as `2026-09-23T00:00:00Z`, or a `YYYY-MM-DD` date meaning midnight UTC); `validate` flags stale concepts and invalid values, and `show`/`list` expose the effective state.
- **Attested Computations (§10)** - the full contract (`runtime`, `parameters`, `computation`, `executor`, `attester`) is parsed and validated: `runtime` is required, the computation must exist inline or as a file, and contract paths that point at missing files are flagged.
- **Actor convention (§7)** - `generated.by` and `verified[].by` are checked against `human:<id>`, `process:<id>`, and `<producer>/<version>` forms.
- **Versioning (§12)** - `toji init` declares `okf_version: "0.2"` in the root `index.md`; `toji index` preserves an existing declaration when regenerating.
- **v0.1 compatibility (§13)** - legacy `timestamp` and `# Citations` are still consumed (with migration warnings), and `generated.at` falls back to `timestamp`.

Graph and backlinks also follow §6.2 path-valued references: `sources[].resource`, `computation`, `executor.resource`, and `attester.resource` that resolve to concepts become derivation edges.

All four reference bundles in the upstream repository (acme_retail, crypto_bitcoin, ga4, stackoverflow) load and validate with zero errors.

## Project status

In daily use on the author's own vault. Commands: `schema`, `init`, `validate`, `lint`, `index`, `list`, `show`, `search`, `new`, `edit`, `toc`, `completion`, `backlinks`, `graph`, `version`. To publish a bundle as a web site, see [okf-export](https://github.com/Mokkori-Tom/okf-export).

Spec fixes from okfcli/okf are picked over when they land there.

## License

Apache 2.0, as [okfcli/okf](https://github.com/okfcli/okf) and the upstream [Google knowledge-catalog](https://github.com/GoogleCloudPlatform/knowledge-catalog) repository.