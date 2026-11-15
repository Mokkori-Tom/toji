---
name: toji
description: >
  ユーザーのノート（OKF バンドル、場所は $TOJI_BUNDLE）を toji コマンドで検索・閲覧・作成・検証する。
  ユーザーの環境・過去の作業・設定・検証結果、このマシンのツールの挙動について聞かれたとき、
  メモやノートを残してと言われたとき、OKF / Obsidian のノートやバンドルを扱うときは、
  自分の知識や grep より先にこれを使う。
---

# toji

ノートは OKF バンドル（Markdown + YAML frontmatter）。操作は必ず `toji` で行い、ファイルを直接 grep・編集しない。

- パイプ経由（Agent からの実行）では出力は常に JSON。エラーは `{"error": {"kind": ...}}`、終了コード 4 は使い方の誤り。
- バンドル引数は省略できる（`$TOJI_BUNDLE`、なければカレントディレクトリ）。
- オプションは推測しない。ここに無いことは `toji schema <command>` で確かめる（`--help` より短く正確）。

## 調べる

```sh
toji search --meta 電力 上限     # まずこれ。タイトル・description・タグ・index.md の一覧行だけを見る
toji search 電力 上限            # 本文まで全文検索。--meta で見つからないとき。語は AND。かな/カナ・全角半角の揺れを吸収
toji search --tag gpu --type Note
toji show notes/gpu              # 本文と frontmatter
toji backlinks notes/gpu         # このページを参照しているページ
```

- 最初は `--meta` で探す。結果は 1 件ずつ `description`（一行要約）付きで短く、本文の言及だけの的外れなページが混ざらない。
  0 件や足りないときは `--meta` を外して本文も検索する。
- 検索結果の `description` / `matches` は抜粋だけ。答える前に必ず `toji show <id>` で本文を読む。一覧だけで「ノートに無い」と判断しない。
- `list` / `search` / `graph` は JSON で最初の 50 件まで返す。`count` が総数、`truncated: true` なら残りがある。
  `--limit 0` で全件取るより、語・`--tag`・`--type` で絞り込む。

## 書く

```sh
toji new notes/<id> --type Note --title "..." --description "一行要約" --tag a,b --body - <<'MD'
本文（Markdown）
MD
```

- 1 回の呼び出しで完成したページを作る。エディタは開かない。
- 既存のページは上書きも削除もしない。`already exists` で止まったら、別の ID にするか既存ページへの追記でよいかをユーザーに聞く。
- 書いたあとは `toji validate` でエラーが無いことを確かめる（警告は直さなくてよい）。

## 点検する

```sh
toji validate        # errors > 0 なら findings の rule と message を見て直す
toji lint            # 推奨フィールドの不足（警告のみ）
toji graph           # node_count, isolated（孤立ページ）など
toji index --dry-run # index.md の更新内容を確かめる（書き込まない）
toji index           # 各フォルダの index.md のリンク一覧を書き直す
```

- `index` は手書きの部分（見出し、その下のフォルダ説明の段落、リンク一覧でない節）を残し、フォルダ・ページ・ファイルのリンク一覧だけを作り直す。
  ページの要約は frontmatter の description、フォルダの要約はそのフォルダの index.md 冒頭の段落。どちらも無ければ index に書いてある要約を残す。
- 結果の `missing_description` は、冒頭の段落が無いフォルダ。段落はユーザーが書くもので、勝手に埋めない。
- `--fill-descriptions` は index の要約を description の無いページの frontmatter に写す。ページを書き換えるので、ユーザーに言われたときだけ使う。
