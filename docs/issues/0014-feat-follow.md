| 項目 | 値 |
| --- | --- |
| タイトル | `feat: フォロー・アンフォローを実装する` |
| ラベル | `feature` |
| マイルストーン | M3 タイムラインと交流 |
| 起票済み | [#34](https://github.com/yama-shu/575-sns/issues/34) |

---

## 背景・目的

[FR-04-02](../requirements/01-requirements.md#fr-04-交流) の「他ユーザーをフォロー・アンフォローできる」を実装する。

M3 の中で**最初に着手する**。バックログの並び（タイムラインが先）から順序を変えている。

| 依存 | 理由 |
| --- | --- |
| フォロー中タイムライン → フォロー | 誰の投稿を集めるかがフォロー関係で決まる |
| ブロック → フォロー | [BR-08](../design/basic/02-domain-model.md#関係に関するルール)（ブロックでフォローを双方向に解除）がフォローの操作を前提とする |

タイムラインを先に作ると、フォローとブロックの条件を後から足すことになる。
**後から絞る実装は忘れられる**（[#30](https://github.com/yama-shu/575-sns/issues/30) でフォロワー限定の可視性を先に入れたのと同じ理由）。

`follows` テーブルは [#20](https://github.com/yama-shu/575-sns/issues/20) で作成済みで、
BR-05（自分をフォローできない）は CHECK 制約として入っている。
[#30](https://github.com/yama-shu/575-sns/issues/30) で参照（`IsFollowing`）だけ実装済み。

## やること

- [ ] `domain` に `Follow` と `FollowRepository` を定義する
- [ ] `infra/postgres` に `FollowRepository` を実装する
- [ ] `usecase` にフォロー・アンフォローを実装する
- [ ] エラーコード `CANNOT_FOLLOW_SELF` / `BLOCKED_USER` を追加する
- [ ] エンドポイントを実装する
  - [ ] `PUT    /api/v1/users/:handle/follow`
  - [ ] `DELETE /api/v1/users/:handle/follow`
- [ ] 単体テストと結合テストを書く
- [ ] 基本設計 05 の `USER_NOT_FOUND` を訂正する（[下記](#user_not_found-は使わない)）

## 完了条件

- [ ] フォローすると `{"following": true, "followers_count": N}` が 200 で返る
- [ ] **すでにフォロー済みでも 200 が返る**（冪等。409 にしない）
- [ ] アンフォローすると `{"following": false, "followers_count": N}` が 200 で返る
- [ ] フォローしていない相手のアンフォローでも 200 が返る（冪等）
- [ ] `followers_count` が実際のフォロワー数と一致する
- [ ] 自分自身のフォローが 422 `CANNOT_FOLLOW_SELF` になる（[BR-05](../design/basic/02-domain-model.md#関係に関するルール)）
- [ ] 自分がブロックしている相手のフォローが 422 `BLOCKED_USER` になる
- [ ] **自分をブロックしている相手のフォローが 404 になる**（[下記](#ブロックされている側には相手が存在しないように見せる)）
- [ ] 存在しない識別名が 404 になる
- [ ] 退会済みの利用者へのフォローが 404 になる
- [ ] 未ログインが 401 になる
- [ ] 同時に2回フォローしても片方が失敗しない
- [ ] usecase の C1 カバレッジ 90% 以上（[NFR-05-04](../requirements/01-requirements.md#nfr-05-保守性運用性)）
- [ ] `gofmt` の差分がなく、`golangci-lint` が 0 issues

## やらないこと

- **フォロー中一覧・フォロワー一覧**（`GET /users/:handle/following` / `followers`）。
  [下記](#一覧は別-issue-に分ける)の理由で別 Issue にする
- ブロック・通報の実装（次の Issue。本 Issue では参照だけ行う）
- タイムライン（フォローの次）
- 通知（FR-04-04・Could）

## 実装上の注意

### 冪等にする

`POST` ではなく `PUT` / `DELETE` を使う（[基本設計 05](../design/basic/05-api.md#交流)）。
「その状態にする」操作であり、何度実行しても結果が同じである。

すでにフォロー済みでも 200 を返す。409 にしない。
「フォローされている状態にする」という要求は満たされているためである。
通信のリトライで二重に実行されても問題が起きない。

実装では `INSERT ... ON CONFLICT DO NOTHING` を使う。
事前に存在確認してから INSERT すると、確認と INSERT のあいだに
同じ操作が挟まったときに主キー違反になる。

### ブロックされている側には相手が存在しないように見せる

[基本設計 05](../design/basic/05-api.md#put-apiv1usershandlefollow) は
「ブロックしている相手」を 422 `BLOCKED_USER` としているが、
**逆向き（自分がブロックされている）の扱いを定めていない**。

422 `BLOCKED_USER` を返すと、ブロックされた事実が分かる。
[BR-10](../design/basic/02-domain-model.md#関係に関するルール)「ブロックされた側は、ブロックされた事実を知らされない」に反する。

404 を返す。退会済み・存在しない識別名と区別がつかないため、事実が漏れない。

| 向き | 応答 | 理由 |
| --- | --- | --- |
| 自分が相手をブロックしている | 422 `BLOCKED_USER` | 自分のブロックは自分が知っている |
| 相手が自分をブロックしている | 404 | 知らせない（BR-10） |

ブロックの可視性ルール全体は次の Issue（通報・ブロック）で扱う。
本 Issue では `blocks` テーブルを参照するだけで、ブロックを作る手段は実装しない。

### `USER_NOT_FOUND` は使わない

[基本設計 05](../design/basic/05-api.md#put-apiv1usershandlefollow) は
相手が存在しない場合を `USER_NOT_FOUND` としているが、
[詳細設計 03 のエラー一覧](../design/detail/03-error-handling.md#2-エラー一覧)に
この値は無く、`NOT_FOUND`（対象が存在しない / 削除済み）が定義されている。

`NOT_FOUND` を使い、基本設計 05 側を訂正する。理由は2つ。

1. 資源ごとにコードを分けると `POST_NOT_FOUND` / `USER_NOT_FOUND` と増え続け、
   クライアントの分岐が資源の数だけ必要になる。**どの資源が無いかは経路が示している**
2. [#30](https://github.com/yama-shu/575-sns/issues/30) の投稿取得はすでに `NOT_FOUND` を返しており、
   同じ API の中で不統一になる

### 退会済みの利用者はフォローできない

`users.status` が `deleted` の利用者は 404 とする。
DB の外部キーは行の存在しか見ないため、アプリケーション側で確かめる。

`suspended`（運営による停止）は 404 にしない。
一時的な状態であり、解除後にフォロー関係が残っているのが自然である。

### `followers_count` は毎回数える

`users` にフォロワー数の列を持たない。`follows` を `COUNT` する。

[投稿の `like_count`](../design/basic/03-database.md) は非正規化しているが、
これはタイムラインで**20件ぶんの集計が毎回走る**ためである。
フォロワー数はプロフィール表示とフォロー操作の応答でしか使わず、
`follows_followee_id_idx` があるため数えても速い。

非正規化すると、更新漏れで実数とずれたときに気づけない。
必要になってから入れる。判断の材料は実測とする。

## 参考

- [基本設計 02 §5: 関係に関するルール](../design/basic/02-domain-model.md#関係に関するルール)
- [基本設計 03 §2: follows / blocks](../design/basic/03-database.md)
- [基本設計 05 §2: 交流](../design/basic/05-api.md#交流)
- [基本設計 05 §3: PUT /api/v1/users/:handle/follow](../design/basic/05-api.md#put-apiv1usershandlefollow)
- [詳細設計 02 §2: api のパッケージ構成](../design/detail/02-class-design.md#2-api-のパッケージ構成)
- [ADR-0005: タイムラインの構築方式](../adr/0005-timeline-strategy.md)

---

## 補足: 一覧は別 Issue に分ける

`GET /users/:handle/following` と `/followers`（[FR-04-03](../requirements/01-requirements.md#fr-04-交流)）は
本 Issue に含めない。ページネーションの設計が未決のためである。

[基本設計 03 §5](../design/basic/03-database.md#5-カーソルページネーション) が定めるカーソル方式は
`posts.id`（`BIGSERIAL`）を前提としている。

```sql
SELECT * FROM posts WHERE id < :cursor ORDER BY id DESC LIMIT 20;
```

`follows` の主キーは `(follower_id, followee_id)` の複合であり、
**単調増加する一意な列が無い**。`created_at` は一意でないため、
同時刻のフォローが複数あるとカーソルが飛ばす・重複する。

カーソルに何を使うかを決める必要があり、フォローの実装とは独立した判断である。
混ぜると両方の完了条件が曖昧になる。
