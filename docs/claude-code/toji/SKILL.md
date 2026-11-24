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
toji new notes/<id> --type Note --title "..." --description "一行要約" --tag a,b --index --body - <<'MD'
本文（Markdown）
MD
```

- 1 回の呼び出しで完成したページを作る。エディタは開かない。
- `--index` を必ず付ける。そのページのフォルダの index.md だけが更新され、新しいページが一覧に載る。付け忘れたときは、
  出力の `index.stale` が true で、`index.update` に打つべきコマンド（`toji index --dir <フォルダ>`）が入っている。
- 既存のページは上書きも削除もしない。`already exists` で止まったら、別の ID にするか既存ページへの追記でよいかをユーザーに聞く。
- 既存のページに足すときは `toji append` を使う（ファイルを直接編集しない）。既にある部分は変わらない。

```sh
toji append notes/<id> --section "関連" --body - <<'MD'   # その節の末尾に足す。--section が無ければページの末尾
- [新しいページ](./new-page.md)
MD
```

  見出しが無い・2 つあるときはエラーになり、ページの見出し一覧が返る。新しく書いたページから既存ページへリンクしたら、
  既存ページ側にも逆向きのリンクを足すか、ユーザーに聞く。
- 既存のページの一部を直す・消すときは `toji replace` を使う（ファイルを直接編集しない）。`--old` の文字列はページの中、
  または `--section` の節の中にちょうど 1 回だけある必要がある。消すときは `--new` の代わりに `--delete`。

```sh
toji replace notes/<id> --old "- 古い行" --new "- 新しい行" --section "記録" --dry-run --diff   # まず差分を見る
toji replace notes/<id> --old - --delete <<'MD'                                                 # 複数行を消す
- 消したい行 1
- 消したい行 2
MD
```

  直してよいのは、自分がこのセッションで書いた部分と、ユーザーが直すように言った部分だけ。それ以外は `--dry-run --diff` の
  差分を見せてから聞く。見つからない・2 回以上あるときはエラーになり、行番号や「節の外にある」「空白だけ違う」が返る。
  frontmatter を壊す変更は書き込まれずにエラーになる。
- 書いたあとは `toji validate --errors-only --id <id>` で、そのページにエラーが無いことを確かめる（警告は直さなくてよい）。
- 見出しの多い長いページは `toji toc <id>` で冒頭に目次を書ける（`<!-- toc -->` の間。何度実行しても作り直すだけ）。

## 点検する

```sh
toji validate --errors-only         # errors > 0 なら findings の rule と message を見て直す（警告は数だけ）
toji validate --id <フォルダ or ID>  # 一部のページだけ。errors・valid・終了コードもその範囲だけ
toji lint                           # 推奨フィールドの不足（警告のみ）
toji graph                          # node_count, isolated（孤立ページ）など
toji index --dir <フォルダ> --dry-run --diff   # そのフォルダの index.md がどう変わるかを差分で見る
toji index --dir <フォルダ>         # そのフォルダの index.md だけを書き直す
```

- ページを足したあとの `index` は、そのページのフォルダだけを `--dir` で直す。引数なしの `toji index` はバンドル全体を書き換えるので、
  ユーザーに頼まれたときだけ使い、先に `--dry-run --diff` で関係ない変更が無いか見る。

- `index` は手書きの部分（見出し、その下のフォルダ説明の段落、リンク一覧でない節）を残し、フォルダ・ページ・ファイルのリンク一覧だけを作り直す。
  ページの要約は frontmatter の description、フォルダの要約はそのフォルダの index.md 冒頭の段落。どちらも無ければ index に書いてある要約を残す。
- 結果の `missing_description` は、冒頭の段落が無いフォルダ。段落はユーザーが書くもので、勝手に埋めない。
- `--fill-descriptions` は index の要約を description の無いページの frontmatter に写す。ページを書き換えるので、ユーザーに言われたときだけ使う。
