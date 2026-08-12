# 575 — 五七五でしか投稿できないSNS

> 言いたいことは、五七五で。

**575** は、投稿本文が五七五（音数律）に収まっていないと投稿できない SNS です。
俳句SNSではありません。季語も切れ字も不要で、投稿する内容は何でも構いません。
制約は「形式」だけです。多少の字余り・字足らずは許容します。

```
     入力: 「今日もまた 会議のための 会議かな」
     判定: ✅ 定型（5 / 7 / 5）→ 投稿可能

     入力: 「今日は疲れた」
     判定: ❌ 破調（7モーラ）→ 投稿不可
```

---

## 現在のステータス

| フェーズ | 状態 |
| --- | --- |
| 要件定義 | ✅ 完了 |
| 基本設計 | ✅ 完了 |
| 詳細設計 | ✅ 完了 |
| 実装 | 🔵 進行中（M2 API 基盤） |

M0（開発基盤）と M1（判定エンジン）は完了しています。
現在は **M2（API 基盤）** の段階で、認証・判定 API・投稿までが動きます。
タイムラインといいね・フォローは M3、画面は M4 です
（[マイルストーン](https://github.com/yama-shu/575-sns/milestones)）。

---

## ドキュメント

ドキュメントの全体地図は **[docs/README.md](docs/README.md)** にあります。
主要な入口は以下の3つです。

| 入口 | 内容 |
| --- | --- |
| [要件定義書](docs/requirements/01-requirements.md) | 何を作るのか。背景・ペルソナ・ユースケース・機能要件・非機能要件 |
| [Architecture Decision Records](docs/adr/README.md) | なぜそう決めたのか。複数案の比較とトレードオフの記録 |
| [Issue 起票ドラフト](docs/issues/README.md) | GitHub Issues に起票する前の下書き置き場 |

---

## リポジトリ構成

```
.
├── README.md                 # このファイル
├── LICENSE
├── compose.yaml              # ローカル実行用（本番相当のイメージで動かす）
├── compose.override.yaml     # 開発用の差分（ホットリロード等。up で自動適用）
├── .github/workflows/        # CI
├── scripts/                  # 省力化スクリプト（check.sh など）
├── .env.example              # 環境変数の雛形。cp して .env を作る
├── web/                      # TypeScript / Next.js — 画面描画・SSR
├── api/                      # Go / Echo — 業務ロジック・永続化
├── prosody/                  # Python / FastAPI — 五七五判定エンジン
└── docs/
    ├── README.md             # ドキュメント地図（ここから読む）
    ├── requirements/         # 要件定義
    ├── design/
    │   ├── basic/            # 基本設計（構成図・ER図・画面遷移・API一覧）
    │   └── detail/           # 詳細設計（アルゴリズム・クラス設計・エラー設計・テスト設計）
    ├── adr/                  # Architecture Decision Record（意思決定の記録）
    └── issues/               # GitHub Issue 起票用ドラフト
```

サービスを3つに分けている理由は [ADR-0002](docs/adr/0002-tech-stack.md) にあります。
要点は、形態素解析の辞書が数十〜数百 MB をメモリに常駐させるため、
判定エンジンを api に同居させると api を増やすたびに辞書が複製されることです。

---

## 開発環境

### 必要なもの

**Docker と Docker Compose だけあれば動きます。** 各言語の処理系をホストへ入れる必要はありません。

| ツール | バージョン | 用途 |
| --- | --- | --- |
| Docker Engine | 24.0 以上 | 全サービスの実行 |
| Docker Compose | v2.21 以上 | 複数サービスの起動 |

> **メモリ**: Docker に **4 GB 以上**を割り当ててください。
> prosody が形態素解析の辞書をメモリへ展開するため、既定の 2 GB では起動に失敗することがあります。

各サービスが使う処理系は以下です。**コンテナ内に閉じている**ため、
ホストへのインストールは不要です（エディタの補完を効かせたい場合のみ入れてください）。

| サービス | 言語 | 主要ライブラリ |
| --- | --- | --- |
| web | TypeScript / Node.js 24 (LTS) | Next.js 16 / React 19 |
| api | Go 1.26 | Echo v4 / pgx v5 |
| prosody | Python 3.13 | FastAPI / SudachiPy + SudachiDict-core |
| db | PostgreSQL 18 | — |

### 構築手順

```bash
git clone git@github.com:yama-shu/575-sns.git
cd 575-sns
cp .env.example .env        # 既定値のままで動くため、編集は任意
docker compose up --build
```

初回はイメージのビルドと辞書の取得で 5〜10 分ほどかかります。2回目以降はキャッシュが効きます。

起動したら http://localhost:3000 を開いてください。
各サービスの疎通状況が表示されます。

環境が壊れたときは、次のコマンドで完全に破棄してから作り直せます。

```bash
docker compose down -v      # コンテナとボリューム（DB のデータ）を削除する
docker compose up --build
```

### 実行方法

| コマンド | 動作 |
| --- | --- |
| `docker compose up` | **開発用**。ソースを編集すると自動で反映される（ホットリロード） |
| `docker compose -f compose.yaml up --build` | **本番相当**。本番と同じイメージで動かす |
| `docker compose logs -f prosody` | 特定サービスのログを追う |
| `docker compose down` | 停止する（DB のデータは残る） |
| `docker compose down -v` | 停止してデータも破棄する |
| `docker compose run --rm cleanup` | 期限切れセッションを削除する（[後述](#期限切れセッションの削除)） |

`docker compose up` は [compose.override.yaml](compose.override.yaml) を自動的に読み込みます。
開発時にだけ必要なもの（ホットリロード用のバインドマウント、デバッグ用のポート公開）は
すべてそちらに寄せてあるため、**本番イメージに開発ツールが混入しません**。

2つのモードは**別のイメージタグ**（`575-web:dev` と `575-web:runtime`）を使います。
同じタグを共有すると、最後にビルドした方のイメージが両モードで使われてしまい、
開発モードで起動したはずが本番イメージ（`node server.js`）が動く、という事故が起きます。

> モードを切り替えるときは、先に `docker compose down` してください。
> 同じホストポートを使うため、両方を同時には起動できません。

### データベースのマイグレーション

スキーマの変更は [api/internal/db/migrations/](api/internal/db/migrations/) の SQL ファイルで管理します。
`docker compose up` を実行すると `migrate` サービスが先に走り、
**未適用のマイグレーションを適用してから api が起動します**。通常は意識する必要はありません。

手動で操作する場合は次のとおりです。

```bash
docker compose run --rm migrate up          # 未適用のものをすべて適用する
docker compose run --rm migrate version     # 現在のバージョンを表示する
docker compose run --rm migrate down        # 直近の1件を巻き戻す
docker compose run --rm migrate down -n 3   # 直近の3件を巻き戻す
docker compose run --rm migrate down -all   # すべて巻き戻す
```

**api の起動時には適用されません。** 本番では api が複数の Pod にスケールするため、
起動時に適用すると Pod 同士でマイグレーションが競合します
（[基本設計 03 §6](docs/design/basic/03-database.md#6-マイグレーション方針)）。
`migrate` は api と同じイメージに入っており、Kubernetes では Job として実行します。

#### 新しいマイグレーションを追加する

```
api/internal/db/migrations/
  NNNNNN_短い英語の説明.up.sql     ← 適用する SQL
  NNNNNN_短い英語の説明.down.sql   ← 巻き戻す SQL（必須）
```

連番は既存の最大値 + 1 です。**down は必ず用意してください。**
SQL はバイナリに埋め込まれる（`embed`）ため、ファイルを配置し直す必要はありません。

> 開発モードでは `go run` で実行するため、SQL を追加したらそのまま反映されます。
> 本番相当モード（`-f compose.yaml`）では `docker compose build migrate` が必要です。

#### 適用が中断された場合

マイグレーションが途中で失敗すると `dirty` 状態になり、以降の適用が拒否されます。
`migrate version` が異常終了して dirty と表示されたら、
スキーマの実態を確認したうえで `schema_migrations` テーブルを手で修正してください。

### 期限切れセッションの削除

サーバー側でセッションを持つため（[ADR-0006](docs/adr/0006-authentication.md)）、
期限切れの行を定期的に削除します。

```bash
docker compose run --rm cleanup
# {"level":"INFO","msg":"期限切れセッションを削除しました","service":"cleanup",
#  "event":"session_cleanup_completed","deleted":3,"duration_ms":67}
```

`docker compose up` では起動しません（`profiles` を付けています）。
ローカルでセッションが溜まっても困らないためです。

**api の中でタイマーは回していません。** 本番では api が複数の Pod にスケールするため、
Pod の数だけ同じ削除が走ります。`migrate` と同じく独立したバイナリにし、
Kubernetes では CronJob として実行します（M5）。

### 公開されるポート

| URL | サービス | 備考 |
| --- | --- | --- |
| http://localhost:3000 | web | |
| http://localhost:8080/readyz | api | 依存先（db / prosody）への疎通状況を返す |
| http://localhost:8000/readyz | prosody | **開発時のみ**公開。本番はクラスタ内部からのみ到達可能 |
| http://localhost:8000/docs | prosody | 判定 API のドキュメント（開発時のみ） |
| `localhost:5432` | db | **開発時のみ**公開 |

疎通はコマンドラインからも確認できます。

```bash
curl -s localhost:8080/readyz | jq
# {"dependencies":{"database":true,"prosody":true},"ready":true}
```

### 判定を試す

```bash
curl -s -X POST localhost:8000/v1/analyze \
  -H 'Content-Type: application/json' \
  -d '{"text":"今日もまた会議のための会議かな"}' | jq
```

```json
{
  "verdict": "teikei",
  "normalized_text": "今日もまた会議のための会議かな",
  "total_mora": 17,
  "segments": [
    { "text": "今日もまた",   "mora": 5, "expected": 5, "diff": 0 },
    { "text": "会議のための", "mora": 7, "expected": 7, "diff": 0 },
    { "text": "会議かな",     "mora": 5, "expected": 5, "diff": 0 }
  ]
}
```

**破調でも 200 が返ります。** 判定を求められて判定を返しているためです。
「破調だから投稿を拒否する」のは 575 の業務ルールで、api の責務です。

```bash
curl -s -X POST localhost:8000/v1/analyze \
  -H 'Content-Type: application/json' \
  -d '{"text":"今日は疲れた"}' | jq -c '{verdict, reason, total_mora}'
# {"verdict":"hacho","reason":"TOO_FEW_MORA","total_mora":7}
```

### 画面を見る

http://localhost:3000 を開きます。M4 で実装した画面は以下です。

| パス | 画面 | 未ログイン |
| --- | --- | --- |
| `/` | S-01 全体タイムライン | ✅ |
| `/home` | S-02 フォロー中タイムライン | ❌ `/login` へ移動 |
| `/posts/:id` | S-03 投稿詳細 | ✅ |
| `/@:handle` | S-04 ユーザーページ | ✅ |
| `/compose` | S-07 投稿作成 | ❌ `/login` へ移動 |
| `/@:handle/following` | S-05 フォロー中一覧 | ✅ |
| `/@:handle/followers` | S-06 フォロワー一覧 | ✅ |
| `/settings/profile` | S-10 プロフィール編集 | ❌ `/login` へ移動 |
| `/settings/blocks` | S-11 ブロック中一覧 | ❌ `/login` へ移動 |
| `/posts/:id/report` | S-12 通報（通常はモーダル） | ❌ `/login` へ移動 |
| `/login` | S-08 ログイン | ✅ |
| `/signup` | S-09 アカウント登録 | ✅ |

**投稿は句ごとに改行して表示します**（[FR-03-06](docs/requirements/01-requirements.md)）。
一行で表示すると五七五であることが視覚的に分からず、
このサービスを使う理由が伝わらないためです（[基本設計 04 §4](docs/design/basic/04-screens.md#4-投稿の表示形式)）。

改行位置は投稿時に保存した値から復元しています。**表示のたびに再判定しません。**
辞書が更新されても過去の投稿の見え方は変わらず、prosody が落ちていても閲覧できます。

過去へ遡るのは「もっと読む」です。スクロールが下端に達すると自動で押されますが、
**ボタンはリンクでもあり、JavaScript が無効でも `?cursor=` で遡れます**
（[FR-03-03](docs/requirements/01-requirements.md) と
[NFR-06-03](docs/requirements/01-requirements.md) の両立）。

**ブラウザは web としか通信しません。** api への中継は web のサーバー側が行い、
利用者の Cookie を転送します（[基本設計 01 §6](docs/design/basic/01-architecture.md#6-サービス間通信)）。
ブラウザから `localhost:8080` を直接呼ぶ作りにはしていません。

### 交流する

投稿カードの投稿者名から**ユーザーページ**へ、「この句を見る」から**投稿詳細**へ行けます。

| 操作 | 場所 | 備考 |
| --- | --- | --- |
| いいね | 投稿カード | 押すと数が変わる。もう一度押すと取り消し |
| フォロー・解除 | ユーザーページ | フォロー中タイムラインに反映される |
| ブロック・解除 | ユーザーページ | 句が0件になる。**フォローは双方向に外れる**（BR-08） |
| 削除 | 投稿詳細（自分の句のみ） | 畳まれた「削除」を開いてから押す |
| 通報 | 投稿カード（他人の句のみ） | 理由を選ぶ。**自分の句には出ない**（BR-07） |

**すべて `<form>` です。** JavaScript を無効にしても押せます（[NFR-06-03](docs/requirements/01-requirements.md)）。

```bash
# JavaScript 無しでいいねを押す（SSR された form を組み立てて送る）
#   → 200 が返り、応答の HTML では ♡ が ♥ に変わっています
```

通報の理由は4つです（スパム・宣伝／嫌がらせ・攻撃／不適切な内容／その他）。
**既定では選ばれていません** — その他のまま送られると運営の判断材料になりません。

同じ句を2回通報すると、**黙って成功にせず**「すでに通報済みです」と伝えます。
届いていないと思って何度も送ることを防ぐためです。

**ブロックした相手のページは見えます。** 句は0件になりますが、
プロフィールは残り「ブロックを解除」が出ます。404 にすると、
そのページから解除できなくなるためです（[#58](https://github.com/yama-shu/575-sns/issues/58)）。

見えない相手・見えない句はすべて 404 の画面になります。
**理由を出し分けません** — 区別すると存在を教えてしまいます
（[BR-10](docs/design/basic/02-domain-model.md#関係に関するルール)）。

### 運営として通報を処理する

**運営権限を与える API はありません。** DB を直接更新します。

```bash
docker compose exec -T db psql -U sns575 -d sns575 \
  -c "UPDATE users SET is_admin = true WHERE handle = 'tarou';"
```

権限を与える経路は、壊れたときの被害が最も大きくなります。運用者がひとりである以上、
手作業で足ります。

```bash
# 未対応の通報（古い順。待たせている順に処理するため）
curl -s -b /tmp/cookies localhost:8080/api/v1/admin/reports | jq -c '.items[0]'

# 対応する（投稿を非表示にする）
curl -s -b /tmp/cookies -X POST localhost:8080/api/v1/admin/reports/10/resolve -w '%{http_code}\n'
# 204

# 却下する（投稿は変わらない）
curl -s -b /tmp/cookies -X POST localhost:8080/api/v1/admin/reports/11/reject -w '%{http_code}\n'
```

**運営でなければ 404 を返します。** 403 にすると、運営向けの経路が存在すること自体を
教えることになります。未ログイン・一般利用者・存在しない経路を、外から区別できません。

| 誰が | 応答 |
| --- | --- |
| 未ログイン | 404 |
| 一般利用者 | 404 |
| 存在しない経路 | 404 |
| 運営 | 200 |

**対応すると、同じ投稿への未対応の通報もまとめて閉じます**
（[基本設計 02 §4](docs/design/basic/02-domain-model.md)）。1件だけ閉じると、
すでに非表示にした投稿の通報が一覧に残り続けます。投稿の非表示化と通報の更新は
1トランザクションで行います。

**処理済みの通報をもう一度処理すると 409 です。** 黙って成功にすると、
別の運営が先に処理したことに気づかないまま二重に判断することになります。

### 一覧を見る

ユーザーページの「フォロー」「フォロワー」の見出しから一覧へ行けます。
ブロック中の一覧は設定（プロフィール編集）から開きます。

**一覧に操作は置いていません。** フォローやブロックの解除はユーザーページで行います。
一覧の中で押すとページごと描き直され、どこを押したのか分からなくなるためです。

**ブロック中の一覧には、ブロックし返してきた相手も残ります。**
消えると解除する手段が無くなるためです。

### プロフィールを変える

ナビゲーションの「設定」から表示名と自己紹介を変えられます（[FR-01-03](docs/requirements/01-requirements.md)）。

```bash
# 自己紹介だけ空にする（表示名には触れない）
curl -s -b /tmp/cookies -X PATCH localhost:8080/api/v1/me/profile \
  -H 'Content-Type: application/json' -d '{"bio":""}' | jq -c
# {"handle":"tarou","display_name":"たろう","bio":""}
```

**省略と空文字を区別します。** 送らなかった項目は変わらず、空文字を送るとその項目が消えます。
一度書いたら消せない自己紹介は、書くこと自体をためらわせるためです。

**アイコンは未対応です。** [FR-01-03](docs/requirements/01-requirements.md) は
表示名・自己紹介・アイコンを挙げていますが、**画像を置く場所が設計されていません**。
`users.avatar_url` の列はありますが、URL を直接入力させる形にはしていません
（任意の外部 URL を画面に埋め込むと、閲覧者の情報が第三者へ渡ります）。
オブジェクトストレージの選定には ADR が要ります。

識別名（`@handle`）とメールアドレスは変えられません。
識別名は[再利用を禁じている](docs/design/basic/02-domain-model.md)ため、変更も同じ問題を持ちます。

### 詠む

ログインすると右上に「詠む」が出ます。押すとモーダルが開きます。

**入力中に判定結果が出ます。** 区切り・各句の読み・モーラ数を必ず見せます。

```
柿くへば     鐘が鳴るなり     法隆寺
カキクヘバ   カネガナルナリ   ホウリュウジ
5音          7音              5音
✅ 定型   全体で17音です。
```

区切りとモーラ数を隠すと、破調と判定されたときに**なぜ弾かれたのか分からず、
直しようがありません**（[基本設計 04 §3](docs/design/basic/04-screens.md#なぜ区切りとモーラ数を必ず見せるのか)）。
読みを見せているのは、システムが「一日」を イチニチ と読んだのか
ツイタチ と読んだのかを利用者が判断できるようにするためです。

| 判定 | 表示 | 詠めるか |
| --- | --- | :---: |
| 定型 | ✅ 定型 | ✅ |
| 許容 | 🔵 字余り / 字足らず（ズレている句を強調） | ✅ |
| 破調 | ⚠️ 破調（あと何音必要か） | ❌ |
| 読み不明 | ❓ 読み方が分かりません（読めなかった語） | ❌ |
| 判定不能 | ⚠️ 判定できません（prosody 障害） | ❌ |

**キーストロークごとには判定しません。** 入力が止まってから 300ms 待ち、
日本語入力の変換確定前は呼びません（[基本設計 04 §3](docs/design/basic/04-screens.md#判定を呼ぶタイミング)）。

**投稿ボタンを無効にするのは体験のためで、防御ではありません。**
api は保存前に必ず再判定します（[FR-02-05](docs/requirements/01-requirements.md)）。
web は判定結果を送らず、api も受け取りません。

「詠む」は `/compose` へのリンクでもあり、**JavaScript が無効でも詠めます**。
その場合は入力中の判定が動かないため、投稿してからサーバー側の判定結果を受け取ります。

### 認証を試す

```bash
# 登録（セッション Cookie が返る）
curl -s -c /tmp/cookies -X POST localhost:8080/api/v1/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{"handle":"yamada","email":"yamada@example.com","password":"correct-horse-battery","display_name":"やまだ"}'

# 本人の情報
curl -s -b /tmp/cookies localhost:8080/api/v1/me

# ログアウト（サーバー側のセッション行を消す）
curl -s -b /tmp/cookies -X POST localhost:8080/api/v1/auth/logout -o /dev/null -w '%{http_code}\n'

# ログアウト後は 401
curl -s -b /tmp/cookies localhost:8080/api/v1/me
```

**ログアウトは次のリクエストから即座に効きます。** サーバー側のセッションを
消すためで、[ADR-0006](docs/adr/0006-authentication.md) が JWT を却下した理由そのものです。

### api 経由で判定を試す

`localhost:8000` は prosody を直接呼びます。api 経由（`localhost:8080`）は
ログインが必要で、prosody が落ちているときの振る舞いが変わります。

```bash
# ログイン済みの Cookie が必要
curl -s -b /tmp/cookies -X POST localhost:8080/api/v1/prosody/check \
  -H 'Content-Type: application/json' \
  -d '{"body":"今日もまた会議のための会議かな"}' | jq -c '{verdict, total_mora}'
# {"verdict":"teikei","total_mora":17}
```

### 投稿を試す

```bash
# 投稿する（判定はサーバー側で行う。クライアントの判定結果は受け取らない）
curl -s -b /tmp/cookies -X POST localhost:8080/api/v1/posts \
  -H 'Content-Type: application/json' \
  -d '{"body":"今日もまた会議のための会議かな"}' | jq -c '{id, verdict, segments}'
# {"id":"1","verdict":"teikei","segments":[{"text":"今日もまた","mora":5},...]}

# 取得（ログイン不要）
curl -s localhost:8080/api/v1/posts/1 | jq -c '{id, body, author}'

# 削除（投稿者本人のみ。論理削除で行は残る）
curl -s -b /tmp/cookies -X DELETE localhost:8080/api/v1/posts/1 -o /dev/null -w '%{http_code}\n'
# 204
```

**破調は 422 で拒否され、保存されません。** 現在の音数を添えて返します。

```bash
curl -s -b /tmp/cookies -X POST localhost:8080/api/v1/posts \
  -H 'Content-Type: application/json' -d '{"body":"今日は疲れた"}' | jq -c .error
# {"code":"PROSODY_HACHO","message":"五七五になっていません",
#  "details":{"reason":"TOO_FEW_MORA","total_mora":7,"verdict":"hacho"}}
```

`verdict` を添えても無視されます。クライアントの判定を信じると、
「判定OK」という嘘を添えるだけで破調が保存できるためです
（[基本設計 01 §4](docs/design/basic/01-architecture.md#なぜ2回判定するのか)）。

### フォローを試す

```bash
# フォローする（PUT。何度実行しても結果が同じ）
curl -s -b /tmp/cookies -X PUT localhost:8080/api/v1/users/bob/follow
# {"following":true,"followers_count":1}

# 解除する
curl -s -b /tmp/cookies -X DELETE localhost:8080/api/v1/users/bob/follow
# {"following":false,"followers_count":0}
```

**すでにフォロー済みでも 200 が返ります。** 「フォローされている状態にする」
という要求は満たされているためで、通信のリトライで二重に実行されても問題が起きません。

ブロックは向きによって応答が変わります。

| 向き | 応答 |
| --- | --- |
| 自分が相手をブロックしている | 422 `BLOCKED_USER` |
| 相手が自分をブロックしている | 404 `NOT_FOUND` |

後者を 422 にすると、ブロックされた事実が分かってしまいます
（[BR-10](docs/design/basic/02-domain-model.md#関係に関するルール)）。
404 なら存在しない識別名・退会済みと区別がつきません。

### タイムラインを試す

```bash
# 全体タイムライン（ログイン不要）
curl -s "localhost:8080/api/v1/timelines/public?limit=20" | jq -c '{count: (.items|length), next_cursor}'
# {"count":5,"next_cursor":null}

# フォロー中タイムライン（ログインが必要）
curl -s -b /tmp/cookies "localhost:8080/api/v1/timelines/home" | jq -c '.items[].author.handle'

# 続きを取る（next_cursor をそのまま渡す）
curl -s "localhost:8080/api/v1/timelines/public?limit=2&cursor=57" | jq -c '[.items[].id]'
```

`OFFSET` ではなくカーソル方式を使っています
（[ADR-0005](docs/adr/0005-timeline-strategy.md)）。
`OFFSET 2000` は 2000 行を読み飛ばすために 2000 行を実際に読むため、
無限スクロールで深いページに進むほど遅くなります。

| パラメータ | 説明 | 既定値 |
| --- | --- | --- |
| `cursor` | この ID より前を取得する | なし（最新から） |
| `limit` | 取得件数。1〜50 | 20 |

`next_cursor` が `null` なら続きはありません。

**フォロー中タイムラインには、フォロー相手の `followers` 限定の投稿も含まれます。**
全体タイムラインは `public` の投稿だけです。

### プロフィールとユーザーの投稿一覧を試す

```bash
# プロフィール（ログイン不要）
curl -s localhost:8080/api/v1/users/hanako | jq -c
# {"handle":"hanako","display_name":"hanako","created_at":"...",
#  "post_count":5,"following_count":0,"follower_count":1,
#  "following":false,"blocking":false}

# ログイン済みなら関係が返る
curl -s -b /tmp/cookies localhost:8080/api/v1/users/hanako | jq -c '{following, blocking}'

# その利用者の投稿（カーソルは他の一覧と同じ）
curl -s "localhost:8080/api/v1/users/hanako/posts?limit=20" | jq -c '{count:(.items|length), next_cursor}'
```

**投稿数は閲覧者から見える数です。** フォロワー限定を含めた総数を返すと、
一覧に出ている件数と合いません。実際に閲覧者を変えて確かめられます。

| 閲覧者 | `post_count` | 一覧 |
| --- | ---: | ---: |
| 未ログイン | 5 | 5 件（すべて `public`） |
| フォロワー | 6 | 6 件（`followers` を含む） |
| フォローしていない他人 | 5 | 5 件 |
| 本人 | 6 | 6 件 |

見えない相手は 404 です。**理由を区別しません**
（[BR-10](docs/design/basic/02-domain-model.md#関係に関するルール)）。

| 状況 | プロフィール | 投稿一覧 |
| --- | --- | --- |
| 相手が自分をブロックしている | 404 | 404 |
| **自分が相手をブロックしている** | **200**（`blocking: true`） | **0 件** |
| 相手が利用停止・退会済み | 404 | 404 |
| 識別名が存在しない | 404 | 404 |

自分がブロックした相手だけ見えるのは、**そのページから解除できるようにする**ためです。
[BR-09](docs/design/basic/02-domain-model.md#関係に関するルール) が隠すよう求めているのは投稿であり、
プロフィールそのものではありません。

### 関係の一覧を試す

```bash
# フォロワー一覧（ログイン不要）
curl -s localhost:8080/api/v1/users/hanako/followers | jq -c '[.items[].handle]'
# ["ken","aoi","tarou"]

# ログイン済みなら、その相手をフォローしているかが返る
curl -s -b /tmp/cookies localhost:8080/api/v1/users/hanako/followers \
  | jq -c '[.items[] | {handle, following}]'

# ブロック中一覧は本人だけ
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/api/v1/me/blocks   # 401
curl -s -b /tmp/cookies localhost:8080/api/v1/me/blocks | jq -c '[.items[].handle]'
```

**閲覧者から見えない相手を一覧に出しません。** 相手が閲覧者をブロックしている場合と、
利用停止・退会した利用者は除きます。出しても開けば 404 になるためです
（[#58](https://github.com/yama-shu/575-sns/issues/58) でプロフィールを 404 にしています）。

```bash
# aoi が ken をブロックしている場合
curl -s -b /tmp/ken localhost:8080/api/v1/users/hanako/followers | jq -c '[.items[].handle]'
# ["ken","tarou"]          ← aoi が消える
curl -s          localhost:8080/api/v1/users/hanako/followers | jq -c '[.items[].handle]'
# ["ken","aoi","tarou"]    ← 未ログインなら残る
```

カーソルは**相手の利用者 ID** です。`follows` と `blocks` に `id` 列は無く、
`created_at` は同時刻の並びが定まらないためです。

### いいねを試す

```bash
# いいねする（PUT。連打しても件数は増えない）
curl -s -b /tmp/cookies -X PUT localhost:8080/api/v1/posts/1/like
# {"liked":true,"like_count":1}

# 取り消す
curl -s -b /tmp/cookies -X DELETE localhost:8080/api/v1/posts/1/like
# {"liked":false,"like_count":0}
```

`posts.like_count` は `likes` から数えれば求まる値ですが、
タイムライン20件の表示ごとに20回の集計が走るのを避けるため非正規化しています
（[基本設計 03 §4](docs/design/basic/03-database.md#4-いいね数の非正規化)）。

**件数は DB の中で加算します**（`like_count = like_count + 1`）。
アプリ側で読んで加算して書き戻すと、同時に2人がいいねしたときに片方が消えます。

見えない投稿にはいいねできません（削除済み・ブロック中・フォロワー限定で非フォロー）。
いずれも 404 を返します。`like_count` の増加から存在を推測されないようにするためです。

### 通報・ブロックを試す

```bash
# 投稿を通報する（POST。同じ投稿への2回目は 409）
curl -s -b /tmp/cookies -X POST localhost:8080/api/v1/posts/1/report \
  -H 'Content-Type: application/json' -d '{"reason":"spam","comment":"宣伝です"}'
# {"id":"1","reason":"spam","status":"pending","created_at":"..."}

# ブロックする（PUT。冪等）
curl -s -b /tmp/cookies -X PUT localhost:8080/api/v1/users/bob/block
# {"blocked":true}
```

**ブロックするとフォロー関係が双方向に消えます**
（[BR-08](docs/design/basic/02-domain-model.md#br-08-の設計意図)）。
ブロックとフォロー解除は1トランザクションで行うため、
「ブロックはできたがフォローが残る」状態は生じません。

ブロックした相手の投稿は **双方向で** 見えなくなります
（[BR-09](docs/design/basic/02-domain-model.md#br-09-は双方向に効く)）。

| 閲覧者 | 投稿 | 結果 |
| --- | --- | --- |
| ブロックした側 | 相手の投稿 | 404 |
| ブロックされた側 | 相手の投稿 | 404 |
| 未ログイン | どちらの投稿も | 200 |

解除すると再び見えますが、**フォロー関係は復活しません。**

### prosody が落ちているときの振る舞いを試す

```bash
docker compose stop prosody

# 判定と投稿は 503。閲覧系は 200 のまま（縮退運転）
curl -s -b /tmp/cookies -X POST localhost:8080/api/v1/prosody/check \
  -H 'Content-Type: application/json' -d '{"body":"今日もまた会議のための会議かな"}'
# {"error":{"code":"PROSODY_UNAVAILABLE","message":"いま詠めません。しばらく経ってからお試しください"}}
curl -s -b /tmp/cookies -o /dev/null -w '%{http_code}\n' localhost:8080/api/v1/me
# 200
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/api/v1/posts/1
# 200（判定結果は保存済みのため、閲覧に prosody は要らない）

# 10 回失敗するとサーキットブレーカーが開き、以降は prosody を呼ばずに即座に返る
for i in $(seq 1 10); do
  curl -s -o /dev/null -b /tmp/cookies -X POST localhost:8080/api/v1/prosody/check \
    -H 'Content-Type: application/json' -d '{"body":"今日もまた会議のための会議かな"}'
done
docker compose logs api | grep prosody_circuit_state_changed
# {"event":"prosody_circuit_state_changed","from":"closed","to":"open"}

docker compose start prosody
# 開放から 30 秒後に半開へ移り、試行が成功すると閉じる
```

### API 定義

3サービスは言語が違い、型定義を共有できません（[ADR-0002](docs/adr/0002-tech-stack.md)）。
その代わりに **API の契約を OpenAPI 定義として管理**しています
（[基本設計 05 §6](docs/design/basic/05-api.md#6-openapi-定義)）。

| 定義 | 対象 | 作り方 |
| --- | --- | --- |
| [api/openapi.yaml](api/openapi.yaml) | api の外部 API | **手で書く**（Go に生成元が無いため） |
| [prosody/openapi.json](prosody/openapi.json) | prosody の内部 API | FastAPI が型ヒントから生成する |
| [web/src/lib/api/schema.d.ts](web/src/lib/api/schema.d.ts) | web の型 | api の定義から生成する |

定義を変えたら次を実行し、生成物をコミットします。

```bash
./scripts/openapi.sh          # 書き出し・検証・型生成
./scripts/openapi.sh --check  # 最新かどうかを確かめる（CI と同じ）
```

**手で書いた定義は、書いた時点から実装とずれ始めます。** prosody は生成物なのでずれませんが、
api にはその保証がありません。そのため
[api/internal/handler/openapi_test.go](api/internal/handler/openapi_test.go) で
**実際の応答が定義に適合すること**を検査しています。

応答のスキーマには `additionalProperties: false` を付けてあります。
実装に項目を足して定義に書き忘れると、テストが落ちます。
api（Go）と web（TypeScript）はこれを契約として型を生成するため、
**定義を変えたらコミットしてください**。CI が最新かどうかを検査します。

```bash
./scripts/openapi.sh          # 書き出す
./scripts/openapi.sh --check  # 最新か確かめる
```

### 設定方法

設定値はコードに埋め込まず、**すべて環境変数**から読み込みます。
ローカルでは [.env](.env.example) で、本番では Kubernetes の Secret / ConfigMap で与えます。

`.env` が無くても [compose.yaml](compose.yaml) の既定値で起動します。変えたい項目だけ書いてください。

| 環境変数 | 既定値 | 説明 |
| --- | --- | --- |
| `POSTGRES_USER` | `sns575` | DB のユーザ名 |
| `POSTGRES_PASSWORD` | `local-dev-only` | DB のパスワード。**ローカル専用の値** |
| `POSTGRES_DB` | `sns575` | DB 名 |
| `POSTGRES_PORT` | `5432` | ホスト側に公開するポート |
| `API_PORT` | `8080` | ホスト側に公開するポート |
| `API_DATABASE_URL` | compose が組み立て | DB 接続文字列。**パスワードを含むためログに出さない** |
| `API_PROSODY_URL` | `http://prosody:8000` | prosody の場所 |
| `API_DATABASE_TIMEOUT` | `3s` | DB のタイムアウト |
| `API_PROSODY_TIMEOUT` | `1s` | prosody のタイムアウト（[基本設計 01 §6](docs/design/basic/01-architecture.md#6-サービス間通信)） |
| `API_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `API_SECURE_COOKIE` | `false` | セッション Cookie に `Secure` を付けるか。**本番では必ず `true`** |
| `API_BCRYPT_COST` | `12` | パスワードのハッシュ化コスト。テストでは下げる |
| `PROSODY_PORT` | `8000` | ホスト側に公開するポート |
| `PROSODY_LOG_LEVEL` | `info` | ログレベル |
| `PROSODY_SUDACHI_DICT` | `core` | 使用する SudachiDict（[ADR-0003](docs/adr/0003-morphological-analyzer.md)） |
| `PROSODY_WORKERS` | `1` | 本番のワーカープロセス数。判定は CPU バウンドのため CPU 数に合わせる |
| `WEB_PORT` | `3000` | ホスト側に公開するポート |
| `WEB_API_INTERNAL_URL` | `http://api:8080` | SSR 時に web が api を呼ぶ先 |

> **認証情報をコミットしないでください**（[NFR-04-07](docs/requirements/01-requirements.md#nfr-04-セキュリティ)）。
> `.env` は [.gitignore](.gitignore) で除外済みです。`.env.example` には実在する値を書かないでください。

### ビルド方法

```bash
docker compose -f compose.yaml build          # 本番相当のイメージを全サービス分
docker compose -f compose.yaml build prosody  # 特定のサービスだけ
```

各 Dockerfile はマルチステージ構成です。`dev` と `runtime` を分けているため、
本番イメージにはコンパイラ・テストツール・ホットリロードの仕組みが入りません。

### テスト方法

判定ロジックは M1 で実装します。現時点では起動確認のテストのみです。
テストケースの設計は [詳細設計 04](docs/design/detail/04-test-design.md) にあります。

```bash
# prosody — pytest（カバレッジは pyproject.toml の設定で常時計測される）
docker compose run --rm prosody pytest

# api — go test（結合テストは API_DATABASE_URL があるときだけ走る）
docker compose exec api go test ./... -cover

# web — 型検査と lint
docker compose run --rm web npm run typecheck
docker compose run --rm web npm run lint
```

カバレッジの目標は [NFR-05-04](docs/requirements/01-requirements.md#nfr-05-保守性運用性) にもとづき、
prosody が C1 100%、api の usecase 層が 90% です。

#### CI と同じ検査をまとめて実行する

lint・型検査・テストを CI と同じ内容で流せます。**push する前にこれを通してください。**

```bash
./scripts/check.sh              # 3サービスすべて
./scripts/check.sh prosody      # サービスを指定
./scripts/check.sh api web
```

使うイメージとツールのバージョンは [CI](.github/workflows/ci.yml) と揃えてあるため、
「手元では通ったのに CI で落ちる」が起きません。

### CI

Pull Request を作ると [CI](.github/workflows/ci.yml) が自動で走ります。

| ジョブ | 内容 |
| --- | --- |
| 変更範囲の判定 | 変更されたサービスだけを実行する。共有の定義が変わった場合は全サービス |
| prosody | ruff / ruff format / mypy / pytest（**C1 100% を下回ると失敗**）/ pip-audit |
| api | gofmt / go vet / golangci-lint / go test / govulncheck |
| web | eslint / tsc / next build / npm audit |
| イメージのビルド | 各サービスの `runtime` を **x86_64 ランナー**でビルドする |
| カバレッジの報告 | 結果を PR に1つのコメントとして投稿・更新する |
| CI | 上記を集約する。ブランチ保護の required status check はこれを指定する |

**イメージのビルドを x86_64 で行う理由**は、本番が ConoHa VPS の x86_64 で動くためです
（[ADR-0007](docs/adr/0007-hosting-conoha-vps.md)）。アーキテクチャが違うランナーで検証すると、
本番でのみ壊れる依存を見逃します。

### リリース方法

本番へのデプロイは M5 で整備します（[未起票のバックログ](docs/issues/README.md#未起票のバックログ)）。
現時点でリリース版のイメージを手元で作る手順は以下です。

```bash
# 本番相当のイメージをタグ付きでビルドする
docker build -t 575-sns/prosody:$(git describe --tags --always) --target runtime ./prosody
docker build -t 575-sns/api:$(git describe --tags --always)     --target runtime ./api
docker build -t 575-sns/web:$(git describe --tags --always)     --target runtime ./web
```

本番は x86_64（ConoHa VPS）で動きます（[ADR-0007](docs/adr/0007-hosting-conoha-vps.md)）。
開発機が Apple Silicon の場合はローカルで ARM のイメージができるため、
本番と同じものを確認したい場合は `--platform linux/amd64` を付けてください。

---

## ライセンス

[MIT License](LICENSE)
