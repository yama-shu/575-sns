| 項目 | 値 |
| --- | --- |
| タイトル | `infra: api の OpenAPI 定義を作り、web の型を生成する` |
| ラベル | `infrastructure` |
| マイルストーン | M4 画面 |
| 起票済み | [#46](https://github.com/yama-shu/575-sns/issues/46) |

---

## 背景・目的

[基本設計 05 §6](../design/basic/05-api.md#6-openapi-定義) が定めた契約の管理を実装する。

> | 対象 | 生成方法 |
> | --- | --- |
> | api（Go） | 定義ファイルを手で書き、サーバーの型を生成する |
> | prosody（Python） | FastAPI が型ヒントから自動生成する |
> | web（TypeScript） | api の定義からクライアントの型を生成する |

prosody の定義は [#8](https://github.com/yama-shu/575-sns/issues/8) で用意し、CI で最新かを検査している。
**api の定義は無い。**

M4 の最初に行う。定義が無いまま画面を作ると、web が手書きの型を持つことになり、
api を変えても web は気づかない。[ADR-0002](../adr/0002-tech-stack.md) が
「言語が3つに分かれるため型定義を共有できない」代償として受け入れたものを、
払わないまま進めることになる。

## やること

- [ ] `api/openapi.yaml` を書く（M3 までに実装したエンドポイント）
- [ ] web で型を生成する仕組みを入れる
- [ ] 生成物をリポジトリで管理し、CI でずれを検出する
- [ ] `scripts/openapi.sh` を api にも対応させる
- [ ] **定義とサーバーの実際の応答が一致することを検査する**（[下記](#定義が実装とずれていないことを確かめる)）
- [ ] README に手順を追記する

## 完了条件

- [ ] M3 までに実装した全エンドポイントが定義に含まれる（[下記](#対象のエンドポイント)）
- [ ] 定義が OpenAPI 3.1 として妥当である（機械的に検証する）
- [ ] web から生成した型で、api の応答を型安全に扱える
- [ ] 生成物が古いと CI が失敗する
- [ ] **実際の応答が定義に適合することを検査している**（少なくとも成功時の応答）
- [ ] エラー応答の形（`error.code` / `message` / `details`）が定義に含まれる
- [ ] 定義の更新手順が README に書かれている
- [ ] `./scripts/check.sh` が通る

## やらないこと

- **Go サーバーの型生成**（`oapi-codegen` 等での置き換え）。
  既存のハンドラをすべて書き換える作業であり、契約を用意することとは別である。
  [下記](#go-の型生成は別-issue-にする)の理由で別 Issue にする
- 未実装のエンドポイント（`/api/v1/admin/*`、一覧系）の定義。
  実装より先に定義を書くと、実装時に必ず食い違う
- web の画面実装（次の Issue から）

## 実装上の注意

### 対象のエンドポイント

M3 までに実装したもの。

| 分類 | エンドポイント |
| --- | --- |
| 認証 | `POST /auth/signup` / `POST /auth/login` / `POST /auth/logout` / `GET /me` |
| 判定 | `POST /prosody/check` |
| 投稿 | `POST /posts` / `GET /posts/:id` / `DELETE /posts/:id` |
| タイムライン | `GET /timelines/public` / `GET /timelines/home` |
| 交流 | `PUT|DELETE /users/:handle/follow` / `PUT|DELETE /posts/:id/like` |
| 健全性 | `POST /posts/:id/report` / `PUT|DELETE /users/:handle/block` |
| 稼働 | `GET /healthz` / `GET /readyz` |

### 定義が実装とずれていないことを確かめる

**手で書く定義は、書いた時点から実装とずれ始める。** prosody は FastAPI が
生成するためずれないが、api は生成元が無い。

生成物のずれ（CI で `openapi.yaml` と生成した型を突き合わせる）は検出できても、
**定義とサーバーの実際の応答のずれは検出できない。**

結合テストで、実際の応答が定義のスキーマに適合することを検査する。
[#8](https://github.com/yama-shu/575-sns/issues/8) で prosody の定義を CI 検査したのと同じ考え方で、
生成できない側は**実物を突き合わせて**担保する。

### Go の型生成は別 Issue にする

[基本設計 05 §6](../design/basic/05-api.md#6-openapi-定義) は「サーバーの型を生成する」としている。
これを行えば定義と実装のずれはコンパイル時に検出できる。

一方、既存のハンドラ（認証・投稿・タイムライン・交流・健全性）の
リクエスト/レスポンス型をすべて生成物に置き換える作業になる。
契約を用意することとは目的が違い、混ぜると両方の完了条件が曖昧になる。

**本 Issue では応答の検査で担保し、型生成は別 Issue とする。**
どちらを先にするかは、応答の検査を入れたうえで判断する。

### 生成に使うもの

| 用途 | 候補 | 備考 |
| --- | --- | --- |
| web の型生成 | `openapi-typescript` | 型だけを生成する。実行時のコードを増やさない |
| 定義の検証 | `redocly` / `swagger-cli` | OpenAPI 3.1 に対応しているものを選ぶ |
| 応答の検証（Go） | `kin-openapi` | 定義を読み、実際の応答を突き合わせる |

**実行時に増えるものを最小にする。** web は型だけを生成し、
HTTP クライアントは生成しない。生成したクライアントは、
Cookie の扱いやエラー処理を独自に持ち込み、[詳細設計 03](../design/detail/03-error-handling.md) の
エラー設計と食い違いやすい。

### 定義は1ファイルにまとめる

分割すると参照の解決が生成ツールごとに変わり、動かない組み合わせが出る。
規模（20 弱のエンドポイント）から見て1ファイルで足りる。

## 参考

- [基本設計 05: API 設計](../design/basic/05-api.md)
- [基本設計 05 §6: OpenAPI 定義](../design/basic/05-api.md#6-openapi-定義)
- [ADR-0002: 技術スタックの選定](../adr/0002-tech-stack.md)
- [詳細設計 03: エラー設計](../design/detail/03-error-handling.md)
- [prosody/openapi.json](../../prosody/openapi.json)（先例）
- [scripts/openapi.sh](../../scripts/openapi.sh)（先例）
