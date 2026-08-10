| 項目 | 値 |
| --- | --- |
| タイトル | `feat: タイムライン取得（全体 / フォロー中）を実装する` |
| ラベル | `feature` |
| マイルストーン | M3 タイムラインと交流 |
| 起票済み | [#38](https://github.com/yama-shu/575-sns/issues/38) |

---

## 背景・目的

[FR-03-01](../requirements/01-requirements.md#fr-03-閲覧)（全体タイムライン）と
[FR-03-02](../requirements/01-requirements.md#fr-03-閲覧)（フォロー中タイムライン）を実装する。

575 で**最も頻繁に実行されるクエリ**であり（[基本設計 03 §3](../design/basic/03-database.md#実行計画の確認を必須とする)）、
[NFR-01-02](../requirements/01-requirements.md#nfr-01-性能)（P95 < 300ms）の対象でもある。

材料は揃っている。

| 依存 | 状態 |
| --- | --- |
| 投稿 | [#30](https://github.com/yama-shu/575-sns/issues/30) |
| フォロー | [#34](https://github.com/yama-shu/575-sns/issues/34) |
| ブロック | [#36](https://github.com/yama-shu/575-sns/issues/36) |
| インデックス #6 #7 #8 #12 | [#20](https://github.com/yama-shu/575-sns/issues/20) |

## やること

- [ ] `domain` にカーソルとページの型を定義する
- [ ] `infra/postgres` に全体・フォロー中の2クエリを実装する
- [ ] `usecase` にタイムライン取得を実装する
- [ ] エンドポイントを実装する
  - [ ] `GET /api/v1/timelines/public`
  - [ ] `GET /api/v1/timelines/home`
- [ ] `liked_by_me` を1クエリで取る（[下記](#liked_by_me-を-n1-にしない)）
- [ ] 基本設計 03 のクエリをブロックの双方向に合わせて訂正する（[下記](#設計書のクエリはブロックが片方向のまま)）
- [ ] 単体テストと結合テストを書く

## 完了条件

### 共通

- [ ] `limit` の既定値が 20、最大が 50 である
- [ ] `limit` が 0 以下・51 以上なら 400 になる
- [ ] `cursor` を省略すると最新から返る
- [ ] `cursor` を指定するとその ID より前だけが返る
- [ ] `cursor` が数値でなければ 400 になる
- [ ] 新しい順（`id DESC`）に並ぶ
- [ ] 続きが無いとき `next_cursor` が `null` になる
- [ ] 続きがあるとき `next_cursor` で次のページを取得でき、**重複も欠落もない**
- [ ] 削除済み・非表示の投稿が含まれない

### 全体タイムライン

- [ ] 未ログインでも取得できる
- [ ] `visibility=followers` の投稿が含まれない
- [ ] ブロック関係にある利用者の投稿が含まれない（双方向。[BR-09](../design/basic/02-domain-model.md#br-09-は双方向に効く)）
- [ ] 未ログインの取得はブロックの影響を受けない

### フォロー中タイムライン

- [ ] 未ログインが 401 になる
- [ ] フォローしている利用者の投稿だけが返る
- [ ] フォロイーの `visibility=followers` の投稿も含まれる
- [ ] フォローを解除すると、その利用者の投稿が消える
- [ ] ブロック関係にある利用者の投稿が含まれない

### 性能

- [ ] 各投稿の `liked_by_me` が**1クエリ**で取れている（N+1 でない）
- [ ] `EXPLAIN ANALYZE` で `posts` に Seq Scan が出ない

### 共通（品質）

- [ ] usecase の C1 カバレッジ 90% 以上（[NFR-05-04](../requirements/01-requirements.md#nfr-05-保守性運用性)）
- [ ] `gofmt` の差分がなく、`golangci-lint` が 0 issues

## やらないこと

- **10万行での実行計画の測定**（次の Issue）。本 Issue では Seq Scan が出ないことだけを確認し、
  規模を伴う測定と記録は `perf` の Issue で行う
- ユーザーページの投稿一覧（`GET /users/:handle/posts`）。別 Issue
- フォロー中一覧・フォロワー一覧・ブロック中一覧（カーソル未決の別 Issue）
- いいね（M3 の最後）
- 無限スクロール（M4 の画面）

## 実装上の注意

### 設計書のクエリはブロックが片方向のまま

[基本設計 03 §3](../design/basic/03-database.md#主要クエリと使われるインデックス) のクエリは
ブロックを片方向でしか除外していない。

```sql
NOT EXISTS (SELECT 1 FROM blocks b
            WHERE b.blocker_id = :me AND b.blocked_id = p.author_id)
```

[#36](https://github.com/yama-shu/575-sns/issues/36) で BR-09 を双方向と定めたため、
**逆向き（投稿者が自分をブロックしている）も除外する**必要がある。
片方向のままだと、ブロックされた側のタイムラインに相手の投稿が流れ続ける。

```sql
NOT EXISTS (SELECT 1 FROM blocks b
            WHERE (b.blocker_id = :me AND b.blocked_id = p.author_id)
               OR (b.blocker_id = p.author_id AND b.blocked_id = :me))
```

設計書を訂正する。インデックス #12（`blocks` の主キー）は
`(blocker_id, blocked_id)` の順であり、**逆向きの検索には効かない**。
実行計画で確認し、必要ならインデックスを追加する。

### `liked_by_me` を N+1 にしない

[#30](https://github.com/yama-shu/575-sns/issues/30) の `IsLikedBy` は投稿1件ごとに1クエリを投げる。
タイムラインで20件ぶん呼ぶと**21回のクエリ**になる。

タイムラインのクエリ内で `EXISTS` として取る。

```sql
EXISTS (SELECT 1 FROM likes l WHERE l.post_id = p.id AND l.user_id = :me) AS liked_by_me
```

`likes` の主キーは `(user_id, post_id)` であり、この検索に効く（インデックス #10）。

### カーソルは `posts.id`

[基本設計 03 §5](../design/basic/03-database.md#5-カーソルページネーション) のとおり `OFFSET` を使わない。
`OFFSET 2000` は 2000 行を読み飛ばすために 2000 行を実際に読むため、
無限スクロール（[FR-03-03](../requirements/01-requirements.md#fr-03-閲覧)）で破綻する。

`next_cursor` は**返した最後の投稿の ID** とする。
`limit` 件に満たなかった場合は `null` を返す。

> 同一ミリ秒の採番順で1件が読み飛ばされうる問題は
> [基本設計 03 §5](../design/basic/03-database.md#id-をカーソルに使うことの前提とリスク) で許容済み。

### 全体タイムラインは未ログインでも見られる

[基本設計 05](../design/basic/05-api.md#タイムライン) のとおり認証は不要。
未ログインの場合はブロックと `liked_by_me` の条件を外す。
誰でもない相手をブロックすることはできず、いいねもしていない。

### フォロー中タイムラインは `visibility` で絞らない

フォローしている相手の `followers` 限定の投稿は**見えるべき**である。
[インデックス #7](../design/basic/03-database.md) が
`visibility` を条件に含めていないのはこのためである（マイグレーションにも記載あり）。

一方、全体タイムラインは `public` だけを返す。

### 自分の投稿は含めない

フォロー中タイムラインは `follows` との JOIN で作るため、自分の投稿は出ない
（BR-05 により自分をフォローできない）。
[基本設計 03 §3](../design/basic/03-database.md#主要クエリと使われるインデックス) のクエリどおりの挙動である。

利用者から見て自然かどうかは画面（M4）で判断できる。
いま挙動を変えると設計書と食い違うため、変えずに進める。
**画面の実装時に再検討する。**

## 参考

- [ADR-0005: タイムラインの構築方式](../adr/0005-timeline-strategy.md)
- [基本設計 03 §3: 主要クエリと使われるインデックス](../design/basic/03-database.md#主要クエリと使われるインデックス)
- [基本設計 03 §5: カーソルページネーション](../design/basic/03-database.md#5-カーソルページネーション)
- [基本設計 05 §1: ページネーション](../design/basic/05-api.md#ページネーション)
- [基本設計 05 §3: GET /api/v1/timelines/home](../design/basic/05-api.md#get-apiv1timelineshome)
- [基本設計 02 §5: BR-09 は双方向に効く](../design/basic/02-domain-model.md#br-09-は双方向に効く)
