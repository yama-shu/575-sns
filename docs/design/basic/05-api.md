# 基本設計 05: API 設計

| 項目 | 内容 |
| --- | --- |
| ドキュメント種別 | 基本設計 |
| 前提 | [基本設計 01](01-architecture.md) / [ADR-0006](../../adr/0006-authentication.md) |
| 最終更新 | 2026-08-06 |

---

## 1. 共通仕様

| 項目 | 内容 |
| --- | --- |
| ベースパス | `/api/v1` |
| 形式 | JSON（`Content-Type: application/json`） |
| 認証 | セッション Cookie（[ADR-0006](../../adr/0006-authentication.md)） |
| 文字コード | UTF-8 |
| 日時形式 | RFC 3339（例: `2026-08-06T12:34:56+09:00`） |

### バージョニング

パスに `v1` を含める。
破壊的変更が必要になった場合は `v2` を並走させ、`v1` を段階的に廃止する。

将来モバイルアプリを作った場合、**古いバージョンのアプリが残り続ける**。
サーバー側の都合で一斉に切り替えることができないため、
最初からバージョンを切れる形にしておく。

### エラーレスポンス

すべてのエラーは同一の形式で返す。

```json
{
  "error": {
    "code": "PROSODY_HACHO",
    "message": "五七五になっていません",
    "details": { }
  }
}
```

| フィールド | 用途 |
| --- | --- |
| `code` | **プログラムが分岐に使う**。文字列の定数。変更しない |
| `message` | **人間が読む**。日本語。文言は変更されうる |
| `details` | エラー固有の追加情報。無い場合は省略 |

`message` で分岐してはならない。文言変更でクライアントが壊れる。
分岐は必ず `code` で行う。

### HTTP ステータスコードの使い分け

| コード | 使う場面 | 例 |
| --- | --- | --- |
| 200 | 取得・更新の成功 | タイムライン取得 |
| 201 | 作成の成功 | 投稿の作成 |
| 204 | 成功したが返す内容がない | いいねの取り消し |
| 400 | リクエストの形式が不正 | JSON が壊れている |
| 401 | 未認証 | ログインしていない |
| 403 | 認証済みだが権限がない | 他人の投稿を削除しようとした |
| 404 | 対象が存在しない | 存在しない投稿 ID |
| 409 | 状態が競合している | すでにフォロー済み |
| 422 | 形式は正しいが内容が処理できない | **破調の本文で投稿しようとした** |
| 429 | レート制限 | 短時間に大量のリクエスト |
| 500 | サーバー内部エラー | 想定外の例外 |
| 503 | 一時的に利用できない | prosody が応答しない |

**破調に 400 ではなく 422 を使う理由**:
リクエストの形式（JSON の構造・型・必須項目）はすべて正しい。
不正なのは「内容が五七五になっていない」という業務ルール上の問題である。
これを 400 にすると、クライアント実装のバグ（形式不正）と
ユーザー入力の問題（破調）を区別できなくなる。

### レート制限

[NFR-04-05](../../requirements/01-requirements.md#nfr-04-セキュリティ) に基づく。

| 対象 | 制限 | 理由 |
| --- | --- | --- |
| `POST /auth/login` | 5回 / 分 / IP | ブルートフォース対策 |
| `POST /auth/signup` | 3回 / 時 / IP | 大量アカウント作成の抑止 |
| `POST /prosody/check` | 60回 / 分 / ユーザー | 判定は CPU バウンドで最も重い |
| `POST /posts` | 20回 / 時 / ユーザー | 連続投稿によるタイムライン占拠の抑止 |
| その他 | 300回 / 分 / ユーザー | 全般的な保護 |

`POST /prosody/check` の 60回/分 は、デバウンス 300ms
（[基本設計 04](04-screens.md#判定を呼ぶタイミング)）を前提とした値である。
正常な入力では毎分数回程度しか発生しない。

### ページネーション

カーソル方式（[ADR-0005](../../adr/0005-timeline-strategy.md) / [基本設計 03 §5](03-database.md#5-カーソルページネーション)）。

**リクエスト**

| パラメータ | 説明 | 既定値 |
| --- | --- | --- |
| `cursor` | この ID より前を取得する。初回は省略 | なし |
| `limit` | 取得件数。最大 50 | 20 |

**レスポンス**

```json
{
  "items": [ ... ],
  "next_cursor": "1234"
}
```

`next_cursor` が `null` なら、それ以上のデータはない。

---

## 2. エンドポイント一覧

### 認証

| メソッド | パス | 認証 | 説明 |
| --- | --- | :---: | --- |
| POST | `/api/v1/auth/signup` | — | アカウント登録 |
| POST | `/api/v1/auth/login` | — | ログイン |
| POST | `/api/v1/auth/logout` | ✅ | ログアウト |
| GET | `/api/v1/me` | ✅ | ログイン中のユーザー情報 |

### 判定

| メソッド | パス | 認証 | 説明 |
| --- | --- | :---: | --- |
| POST | `/api/v1/prosody/check` | ✅ | 五七五判定（保存しない） |

### 投稿

| メソッド | パス | 認証 | 説明 |
| --- | --- | :---: | --- |
| POST | `/api/v1/posts` | ✅ | 投稿する |
| GET | `/api/v1/posts/:id` | — | 投稿を取得する |
| DELETE | `/api/v1/posts/:id` | ✅ | 投稿を削除する |

### タイムライン

| メソッド | パス | 認証 | 説明 |
| --- | --- | :---: | --- |
| GET | `/api/v1/timelines/public` | — | 全体タイムライン |
| GET | `/api/v1/timelines/home` | ✅ | フォロー中タイムライン |

### ユーザー

| メソッド | パス | 認証 | 説明 |
| --- | --- | :---: | --- |
| GET | `/api/v1/users/:handle` | — | プロフィール |
| GET | `/api/v1/users/:handle/posts` | — | そのユーザーの投稿一覧 |
| PATCH | `/api/v1/me/profile` | ✅ | プロフィール更新 |
| DELETE | `/api/v1/me` | ✅ | 退会 |

### 交流

| メソッド | パス | 認証 | 説明 |
| --- | --- | :---: | --- |
| PUT | `/api/v1/posts/:id/like` | ✅ | いいねする |
| DELETE | `/api/v1/posts/:id/like` | ✅ | いいねを取り消す |
| PUT | `/api/v1/users/:handle/follow` | ✅ | フォローする |
| DELETE | `/api/v1/users/:handle/follow` | ✅ | フォローを外す |
| GET | `/api/v1/users/:handle/following` | — | フォロー中一覧 |
| GET | `/api/v1/users/:handle/followers` | — | フォロワー一覧 |

いいね・フォローに **`POST` ではなく `PUT` を使う**。
これらは「その状態にする」操作であり、何度実行しても結果が同じ（冪等）である。
通信のリトライで二重に実行されても問題が起きない。

### 健全性

| メソッド | パス | 認証 | 説明 |
| --- | --- | :---: | --- |
| POST | `/api/v1/posts/:id/report` | ✅ | 投稿を通報する |
| PUT | `/api/v1/users/:handle/block` | ✅ | ブロックする |
| DELETE | `/api/v1/users/:handle/block` | ✅ | ブロックを解除する |
| GET | `/api/v1/me/blocks` | ✅ | ブロック中一覧 |

### 運営

| メソッド | パス | 認証 | 説明 |
| --- | --- | :---: | --- |
| GET | `/api/v1/admin/reports` | 運営 | 未対応の通報一覧 |
| POST | `/api/v1/admin/reports/:id/resolve` | 運営 | 通報に対応する（投稿を非表示化） |
| POST | `/api/v1/admin/reports/:id/reject` | 運営 | 通報を却下する |
| POST | `/api/v1/admin/users/:handle/suspend` | 運営 | 利用を停止する |

---

## 3. 主要エンドポイントの詳細

### POST /api/v1/prosody/check

入力中の判定に使う。**何も保存しない。**

**リクエスト**

```json
{ "body": "今日もまた会議のための会議かな" }
```

**レスポンス（200）**

```json
{
  "verdict": "teikei",
  "reading": "キョウモマタカイギノタメノカイギカナ",
  "total_mora": 17,
  "segments": [
    { "text": "今日もまた",   "reading": "キョウモマタ",   "mora": 5, "expected": 5, "diff":  0 },
    { "text": "会議のための", "reading": "カイギノタメノ", "mora": 7, "expected": 7, "diff":  0 },
    { "text": "会議かな",     "reading": "カイギカナ",     "mora": 5, "expected": 5, "diff":  0 }
  ]
}
```

**レスポンス（200・破調の場合）**

破調は**エラーではない**。判定を求められて判定を返しているため 200 で返す。

```json
{
  "verdict": "hacho",
  "reading": "キョウハツカレタ",
  "total_mora": 8,
  "segments": null,
  "reason": "TOO_FEW_MORA"
}
```

`segments` が `null` なのは、五七五に区切れないため区切りが定義できないからである。

| `verdict` | 意味 | 投稿 |
| --- | --- | :---: |
| `teikei` | 定型 | 可 |
| `kyoyo` | 許容 | 可 |
| `hacho` | 破調 | 不可 |
| `unknown` | **読みを確定できず判定できない** | 不可 |

`reason`（`hacho` / `unknown` のとき）

| 値 | `verdict` | 意味 |
| --- | --- | --- |
| `TOO_FEW_MORA` | hacho | 総モーラ数が少なすぎる |
| `TOO_MANY_MORA` | hacho | 総モーラ数が多すぎる |
| `NO_VALID_SPLIT` | hacho | モーラ数は範囲内だが、許容範囲に収まる区切りが見つからない |
| `READING_UNAVAILABLE` | unknown | 読みを取得できない語が含まれる |

`NO_VALID_SPLIT` は「17音あるが五七五に切れない」場合である。
ユーザーには「音の数は合っていますが、五七五の区切りになっていません」と案内する。

**`unknown` を `hacho` と区別する理由**:
読めなかっただけで「五七五になっていません」と伝えるのは誤りである。
ユーザーは正しく詠んでいるかもしれず、そう言われても直しようがない。
`unknown` のときは `unreadable` に読めなかった語を列挙して返す。

```json
{
  "verdict": "unknown",
  "reason": "READING_UNAVAILABLE",
  "unreadable": ["甃"],
  "reading": null,
  "total_mora": null,
  "segments": null
}
```

詳細は[詳細設計 01 §4](../detail/01-prosody-algorithm.md#4-読みが取得できない場合)を参照。

### POST /api/v1/posts

**リクエスト**

```json
{
  "body": "今日もまた会議のための会議かな",
  "visibility": "public"
}
```

判定結果を**クライアントから受け取らない**。
[基本設計 01 §4](01-architecture.md#なぜ2回判定するのか) のとおり、
サーバー側で必ず再判定する。

**レスポンス（201）**

```json
{
  "id": "1234",
  "body": "今日もまた会議のための会議かな",
  "verdict": "teikei",
  "segments": [
    { "text": "今日もまた",   "mora": 5 },
    { "text": "会議のための", "mora": 7 },
    { "text": "会議かな",     "mora": 5 }
  ],
  "visibility": "public",
  "like_count": 0,
  "liked_by_me": false,
  "author": { "handle": "yamada", "display_name": "やまだ", "avatar_url": null },
  "created_at": "2026-08-06T12:34:56+09:00"
}
```

**レスポンス（422・破調）**

```json
{
  "error": {
    "code": "PROSODY_HACHO",
    "message": "五七五になっていません",
    "details": {
      "verdict": "hacho",
      "total_mora": 8,
      "reason": "TOO_FEW_MORA"
    }
  }
}
```

**レスポンス（422・読みを確定できない）**

```json
{
  "error": {
    "code": "PROSODY_UNKNOWN_READING",
    "message": "読み方が分からない語が含まれています",
    "details": {
      "verdict": "unknown",
      "unreadable": ["甃"]
    }
  }
}
```

**レスポンス（503・prosody 障害）**

```json
{
  "error": {
    "code": "PROSODY_UNAVAILABLE",
    "message": "いま詠むことができません。しばらくしてからお試しください"
  }
}
```

[基本設計 01 §5](01-architecture.md#5-縮退運転) の縮退運転に対応する。
閲覧系の API はこの状況でも 200 を返し続ける。

### GET /api/v1/timelines/home

**リクエスト**

```
GET /api/v1/timelines/home?cursor=1234&limit=20
```

**レスポンス（200）**

```json
{
  "items": [ /* 投稿オブジェクトの配列 */ ],
  "next_cursor": "1214"
}
```

ブロック中のユーザーの投稿は含まれない（[BR-09](02-domain-model.md#関係に関するルール)）。
除外はクエリ内で行い、事前計算しない（[ADR-0005](../../adr/0005-timeline-strategy.md)）。

### PUT /api/v1/users/:handle/follow

**レスポンス（200）**

```json
{ "following": true, "followers_count": 42 }
```

すでにフォロー済みでも **200 を返す**（冪等）。409 にしない。
「フォローされている状態にする」という要求は満たされているためである。

ただし以下はエラーになる。

| 状況 | コード | `error.code` |
| --- | --- | --- |
| 自分自身をフォロー | 422 | `CANNOT_FOLLOW_SELF`（[BR-05](02-domain-model.md#関係に関するルール)） |
| ブロックしている相手 | 422 | `BLOCKED_USER` |
| 相手が存在しない | 404 | `USER_NOT_FOUND` |

---

## 4. prosody の内部 API

api からのみ呼ばれる。**外部に公開しない**（[基本設計 01 §6](01-architecture.md#prosody-に認証を設けない理由)）。

### POST /v1/analyze

**リクエスト**

```json
{ "text": "今日もまた会議のための会議かな" }
```

**レスポンス（200）**

`/api/v1/prosody/check` のレスポンスとほぼ同一。
api はこれをほぼそのまま転送する。

### GET /healthz

Kubernetes の liveness probe 用。プロセスが生きているかだけを返す。

### GET /readyz

Kubernetes の readiness probe 用。
**辞書のロードが完了しているかを返す。**

```json
{ "ready": true, "dictionary_loaded": true }
```

prosody は起動時に辞書をメモリへ展開する。
この間はリクエストを処理できない。
`readyz` が `false` を返す間は、Service がその Pod にトラフィックを流さない。

これを実装しないと、**Pod の再起動やスケールアウトのたびに、
辞書ロード中の Pod へリクエストが流れてタイムアウトする**。

---

## 5. api → prosody 呼び出しの扱い

| 項目 | 設定 | 理由 |
| --- | --- | --- |
| タイムアウト | 1秒 | [NFR-01-01](../../requirements/01-requirements.md#nfr-01-性能) の 150ms に対して十分な余裕。これを超えるのは異常 |
| リトライ | 1回のみ | 判定は副作用がないため安全にリトライできる。ただし過負荷時にリトライで追い打ちをかけないよう1回に限る |
| サーキットブレーカー | 連続失敗で開放 | prosody が死んでいるときに api のスレッドを浪費しない |

詳細な挙動は[詳細設計 03: エラー設計](../detail/03-error-handling.md)で扱う。

---

## 6. OpenAPI 定義

[ADR-0002](../../adr/0002-tech-stack.md) で「言語が3つに分かれるため型定義を共有できない」ことを
デメリットとして受け入れた。その対処として、
**API の契約を OpenAPI 定義としてリポジトリで管理する**。

| 対象 | 生成方法 |
| --- | --- |
| api（Go） | 定義ファイルを手で書き、サーバーの型を生成する |
| prosody（Python） | FastAPI が型ヒントから自動生成する |
| web（TypeScript） | api の定義からクライアントの型を生成する |

これにより「api が返すもの」と「web が期待するもの」のずれを、
実行時ではなくビルド時に検出できる。

---

## 関連ドキュメント

- [基本設計 01: システム構成](01-architecture.md)
- [基本設計 02: ドメインモデルと状態遷移](02-domain-model.md)
- [基本設計 03: データベース設計](03-database.md)
- [基本設計 04: 画面設計](04-screens.md)
- [詳細設計 03: エラー設計](../detail/03-error-handling.md)
