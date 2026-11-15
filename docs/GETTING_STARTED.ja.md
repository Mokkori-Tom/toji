# はじめかた

[toji](https://github.com/Mokkori-Tom/toji)(綴じ)は、Markdown のノートの入ったフォルダを扱うコマンドです。
ノートを作り、日本語で探し、端末で読むことが CLI だけでできます。
エージェントやスクリプトからは JSON で使えます。
ノートの形式は OKF(YAML frontmatter 付きの `.md`)で、Obsidian の vault がそのまま使えます。
元は OKF の検証ツール okfcli/okf のフォークで、その機能はすべて残っています。

## 1. 入れる

Go 1.27 以上が必要です。

```bash
go install github.com/Mokkori-Tom/toji/cmd/toji@latest
# または clone してビルド
git clone https://github.com/Mokkori-Tom/toji.git
cd toji
go build -o ~/.local/bin/toji ./cmd/toji      # Linux / macOS
go build -o $PREFIX/bin/toji ./cmd/toji       # Termux
```

Termux の Go が古いときは、PC でビルドして持っていきます。

```bash
CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -o toji-android-arm64 ./cmd/toji
# できたファイルを Termux の $PREFIX/bin/toji に置き、chmod +x する
```

更新は `go install ...@latest` をもう一度実行するか、clone なら `git pull` してから同じ `go build` を実行します。

## 2. ノートの置き場所(bundle)を決める

bundle は、ノート(`.md`)を入れるディレクトリです。既にある Obsidian の vault をそのまま使えます。
シェルの設定に書いておくと、どこからでも bundle を省略できます。
zsh なら `~/.zshrc` ではなく `~/.zshenv` に書きます。エージェントやスクリプトが使う非対話のシェルにも届くためです(bash は `~/.bashrc`)。

```bash
export TOJI_BUNDLE="$HOME/notes"
```

(toji と呼ばれる前の `OKF_BUNDLE` などの `OKF_*` も、そのまま読みます。)

Tab でページ名・タグ・type・オプションを補完できるように、補完スクリプトも読み込んでおきます。
zsh(oh-my-zsh や `compinit` の後)と bash に対応しています。

```bash
toji completion zsh >/dev/null 2>&1 && source <(toji completion zsh)
# bash なら: toji completion bash >/dev/null 2>&1 && source <(toji completion bash)
```

前半で補完に対応しているかを確かめるので、toji が入っていない環境では何もしません。

`TOJI_BUNDLE` が無いときは、今いるディレクトリが bundle になります。
symlink の先にある bundle も、そのままの path で使えます。

## 3. 使う

```bash
toji new 日記/2026-10-22 --tag 日記     # ページを作って $EDITOR で開く(上書きはしない)
toji new gpu-fan --title "GPUのファン調整" --description "電力上限の記録"
toji new 物理/Kapitsa --type Person --title カピッツァ --tag 物理 --body - < 本文.md
                                       # 本文ごと1回で作る(--body "文字列" も可。エディタは開かない)
                                       # 本文の先頭に --- で囲んだ frontmatter を書くと、そのキー
                                       # (created・sources など)も足される。同じキーはフラグが優先
toji search 電力 上限                   # 部分一致・AND(語を足すほど絞られる)。全角スペース区切りも可。
                                       # ひらがな/カタカナ、全角/半角の英数字の違いを吸収。
                                       # 空白を含む語句そのものを探すときは --text "語 句"
toji search --meta 電力                 # タイトル・description・タグ・index.md の一覧行だけを探す。
                                       # 本文で触れているだけのページは出ず、結果に一行要約が付く
toji search --tag 日記                  # タグで絞る(--type も使える)
toji show gpu-fan                       # ID で読む
toji show 日記/2026-10-22.md            # ファイルの path でも読める
toji edit gpu-fan                       # エディタで開く(show の画面で v を押しても同じ)
toji new 2026-10-27 --template daily    # テンプレートから作る(下の「テンプレート」を参照)
toji list                               # 一覧
toji backlinks gpu-fan                  # このページへリンクしているページ
toji validate                           # OKF の規格どおりかを検査
toji new -h                             # どのコマンドも -h / --help でヘルプ(toji help new も同じ)
```

- 端末での `toji show` は、ページを整形して表示します。見出しの `#` は消し、箇条書き・チェックボックス・
  Obsidian の callout・コードブロック・表(日本語の幅で揃え、入りきらなければ1行ずつ「見出し: 値」の形に)・
  太字やリンクを描きます。折り返しは日本語の禁則に従います。
  長いページは `$TOJI_PAGER`、次に `$PAGER`、無ければ `less` で開きます。
  書いたままの Markdown を見たいときは `--raw`、ページャーを使わないときは `--no-pager` を付けます。
  ページャーが less のときは、`v` を押すと表示中のページをエディタで開けます。
- 端末では人が読みやすい表示になり、パイプやエージェントからは JSON が出ます。
  `--human` / `--json`、または `TOJI_OUTPUT=human|json` で切り替えを固定できます。
  `toji show x | less` のようにパイプへ流すと JSON になるので、人が読むときは `--human` を付けます。
- エディタは `$VISUAL`、次に `$EDITOR`、無ければ nvim・vim・vi・nano の順で探します。
- `toji new` の作者は `$TOJI_AUTHOR`、無ければ `human:$USER` が `generated.by` に入ります。

### テンプレート

`toji new <ページ> --template <名前>` で、テンプレートからページを作ります。
テンプレートは `$TOJI_TEMPLATES`、無ければ bundle の `Templates/`(無ければ `templates/`)に置いた `.md` です。
名前の代わりにファイルの path も渡せます。`--template` の後ろは Tab で補完できます。

テンプレートの中の次の書き方が置き換わります(Obsidian の「テンプレート」と同じ書き方に、日付のずらしを足したもの)。

| 書き方 | 例(2026-10-26 06:08) |
|---|---|
| `{{title}}` | ページのタイトル |
| `{{date}}` / `{{time}}` | `2026-10-26` / `06:08` |
| `{{date:YYYY年M月D日 (ddd)}}` | `2026年10月26日 (Mon)` |
| `{{date+1d:YYYY-MM-DD}}`・`{{date-1w}}` | 1日後・1週前(単位は d・w・M・y) |

ページ名が `2026-10-27` のような日付なら、その日を基準にします(デイリーノートの前日・翌日リンクが合うように)。
それ以外は今日が基準です。テンプレートの frontmatter も合流し、優先順はテンプレート < `--body` < フラグです。
`--body` を一緒に渡すと、テンプレートの本文の後ろに足されます。

## 4. Obsidian の vault から移るとき

書き換えなくても、そのまま list・search・show できます。

| ノートの状態 | 読み込み・検索 | `toji validate` |
|---|---|---|
| frontmatter なし | 使える | 警告1件(`okf/frontmatter/missing`) |
| frontmatter はあるが `type` なし | 使える | 警告1件(`okf/frontmatter/type-required`) |
| `type` あり | 使える | 規格どおり厳密に検査 |
| frontmatter の YAML が壊れている | 使える | エラー(`okf/frontmatter/invalid`) |
| 読めないファイル | 飛ばす(stderr に警告) | エラー(`okf/bundle/unreadable`) |

タイトルが無いノートは、本文の最初の `# 見出し`、それも無ければファイル名で表示します。
`[[ページ名]]` 形式のリンクも、`toji backlinks`・`toji graph` で数えます。
OKF の規格の外なので `toji validate` は検査しません(まだ書いていないページへのリンクは警告になりません)。
名前だけのリンクは Obsidian と同じく vault のどこからでも探し、同じ名前が複数あれば同じフォルダ、次に短いパスを選びます。
OKF に寄せたいノートから順に `type`(例: `Note`)と `title` を書き足していけば十分です。

上の寛容さは、このフォークが足したものです。bundle が OKF として正しいかを確かめたいときは
`toji validate --strict` を使います。frontmatter や `type` の無いノートはエラーになり、
各項目の検査も行い、`[[wikilink]]` もページごとに警告として出します(`toji lint --strict` も同じ)。
`status` は `draft`・`stable`・`deprecated` のどれかにします。
独自の進み具合を残したいときは、`progress:` など別のキーに書きます(OKF は独自キーを許しています)。

## 5. PicoClaw から使う

[picoclaw/AGENT.md](picoclaw/AGENT.md) を PicoClaw の `workspace/AGENT.md` に置き、`TOJI_BUNDLE` を設定して起動します。
PicoClaw の既定の文面に 5 行(toji を使うこと、使い方の調べ方、既存ノートを消さないこと、環境についての質問はまずノートを調べること、本文まで読むこと)を足したものです。
最後の行は外さないでください。これが無いと、`toji new` が「already exists」で止まったときに、
モデルが既存のノートを `rm` で消して作り直すことがあります(Qwen3.6-35B-A3B で 3 回中 2 回、この行で 0 回)。
後ろの 2 行が無いと、一般論でも答えられそうな質問(サーバーの止め方など)ではノートを見ずに答えます
(同じモデルで、ノートにしか無い答えの質問 8 問×3 回が 14/24、2 行ありで 20/24)。

## 6. 困ったとき

- **何も見つからない / `validate` が `okf/bundle/empty` で失敗する**:bundle の path が違います。
  `toji list --human` で件数を確かめてください。
- **`search` の第1引数が検索語にならない**:第1引数が既存のディレクトリか、`/` を含むか `.`・`~` で始まると
  bundle の path として扱います。検索語だけを書くときは `TOJI_BUNDLE` を設定します。
- **規格の詳細**:[OKF SPEC](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md)、
  コマンドの機械向け説明は `toji schema`。
