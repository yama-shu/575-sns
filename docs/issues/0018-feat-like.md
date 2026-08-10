| 項目 | 値 |
| --- | --- |
| タイトル | `feat: いいねを実装する（アトミックな件数更新）` |
| ラベル | `feature` |
| マイルストーン | M3 タイムラインと交流 |
| 起票済み | [#43](https://github.com/yama-shu/575-sns/issues/43) |

---

## 背景・目的

[FR-04-01](../requirements/01-requirements.md#fr-04-交流)（投稿にいいねできる。取り消しもできる）を実装する。
M3 の最後の機能。

`posts.like_count` は非正規化された値であり、[基本設計 03 §4](../design/basic/03-database.md#4-いいね数の非正規化) が
**`likes` とずれる可能性を抱え込むことを明示して受け入れている**。
その代償を実装で払う部分である。

参照側（`liked_by_me`）は [#30](https://github.com/yama-shu/575-sns/issues/30) と
[#38](https://github.com/yama-shu/575-sns/issues/38) で実装済みで、書き込み側だけが無い。

## やること

- [ ] `domain` に `LikeRepository` を定義する
- [ ] `infra/postgres` に実装する（[アトミックな更新](#read-modify-write-で書かない)）
- [ ] `usecase` にいいね・取り消しを実装する
- [ ] エンドポイントを実装する
  - [ ] `PUT    /api/v1/posts/:id/like`
  - [ ] `DELETE /api/v1/posts/:id/like`
- [ ] 単体テストと結合テストを書く
- [ ] `likes` と `like_count` のずれを検出できるようにする（[下記](#ずれを検出する手段を用意する)）

## 完了条件

- [ ] いいねすると 200 が返り、`like_count` が 1 増える
- [ ] **すでにいいね済みでも 200 が返り、`like_count` は増えない**（冪等）
- [ ] 取り消すと `like_count` が 1 減る
- [ ] いいねしていない投稿の取り消しでも 200 が返り、`like_count` は減らない（冪等）
- [ ] `likes` の行と `like_count` が**同一トランザクション**で更新される
- [ ] **同時に 50 件のいいねを投げても `like_count` が 50 になる**（[下記](#read-modify-write-で書かない)）
- [ ] 削除済み・非表示の投稿へのいいねが 404 になる
- [ ] ブロック関係にある投稿へのいいねが 404 になる（[BR-09](../design/basic/02-domain-model.md#br-09-は双方向に効く)）
- [ ] フォロワー限定の投稿に、フォローしていない利用者がいいねできない
- [ ] 自分の投稿にいいねできる（[下記](#自分の投稿へのいいねを禁じない)）
- [ ] 未ログインが 401 になる
- [ ] 存在しない投稿が 404、数値でない ID が 400 になる
- [ ] `like_count` が負にならない（DB の CHECK 制約で担保）
- [ ] usecase の C1 カバレッジ 90% 以上（[NFR-05-04](../requirements/01-requirements.md#nfr-05-保守性運用性)）
- [ ] `gofmt` の差分がなく、`golangci-lint` が 0 issues

## やらないこと

- **突合バッチ**（`likes` の実数と `like_count` の照合・補正）。
  [基本設計 03 §4](../design/basic/03-database.md#4-いいね数の非正規化) が対策として挙げているが、
  ずれが起きうる実装になっていることを確かめてから作る。本 Issue では**検出できる形**にとどめ、
  補正の運用は M5 の運用整備でまとめて扱う
- いいねした利用者の一覧（`GET /posts/:id/likes`）。要件に無い
- 通知（FR-04-04・Could）
- タイムラインの並び替えへの反映（いいね順のタイムラインは要件に無い）

## 実装上の注意

### read-modify-write で書かない

[基本設計 03 §4](../design/basic/03-database.md#4-いいね数の非正規化) が名指しで警告している。

```sql
-- ❌ アプリ側で読んで加算して書き戻す
SELECT like_count FROM posts WHERE id = ?;   -- 10 を読む
UPDATE posts SET like_count = 11 WHERE id = ?;

-- ✅ DB の中で加算する
UPDATE posts SET like_count = like_count + 1 WHERE id = ?;
```

人気の投稿には同時に複数のいいねが飛ぶ。
read-modify-write だと、同時に2人がいいねしたときに片方が消える。

**「消えないこと」をテストで確かめる。** 実装を読んで正しそうに見えても、
同時実行の失敗は目視では見つからない。並行して大量のいいねを投げ、
`like_count` が投げた数と一致することを確認する。

### 冪等にする

`PUT` / `DELETE` を使う（[基本設計 05 §2](../design/basic/05-api.md#交流)）。
すでにいいね済みなら 200 を返し、**`like_count` は増やさない**。

```sql
INSERT INTO likes (user_id, post_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING
```

`ON CONFLICT DO NOTHING` は既存行があると 0 行を返す。
**この戻り値を見て `like_count` の更新を分岐する。**
分岐しないと、同じ利用者が連打するだけで件数が増える。

取り消しも同様に、`DELETE` の影響行数を見て分岐する。

### 1トランザクションで行う

`likes` の更新と `like_count` の更新を必ず同一トランザクションで実行する。
片方だけ成功すると、[基本設計 03 §4](../design/basic/03-database.md#非正規化の代償) が
代償として挙げた「ずれ」がそのまま起きる。

[#36](https://github.com/yama-shu/575-sns/issues/36) のブロック（BR-08 の双方向解除）と同じ方式にする。

### ずれを検出する手段を用意する

補正のバッチは作らないが、**ずれていることに気づけない状態にはしない。**

結合テストで、いいね・取り消しを一通り行ったあとに
`likes` の実数と `like_count` が一致することを確かめる。

```sql
SELECT p.id, p.like_count, count(l.*)
FROM posts p LEFT JOIN likes l ON l.post_id = p.id
GROUP BY p.id HAVING p.like_count <> count(l.*)
```

このクエリが 0 行であることをテストで固定する。
実装がずれを生むようになれば、テストが落ちる。

### いいねできる投稿は「見える投稿」

[#36](https://github.com/yama-shu/575-sns/issues/36) の通報と同じ扱いにする。

| 状況 | 応答 |
| --- | --- |
| 削除済み・非表示 | 404 |
| ブロック関係にある（双方向） | 404 |
| フォロワー限定で、フォローしていない | 404 |

**見えない投稿にいいねできてはならない。** できると、`like_count` の増加から
存在と活動が推測できてしまう。

`usecase.Post.Get` と同じ判定を使う。判定を2箇所に書くと、片方だけ直したときに食い違う。

### 自分の投稿へのいいねを禁じない

[BR-05〜BR-10](../design/basic/02-domain-model.md#関係に関するルール) に
自分の投稿へのいいねを禁じるルールは無い。
要件（FR-04-01）にも制限が書かれていない。

**禁じない。** 通報（BR-07）やフォロー（BR-05）とは違い、
自分でいいねしても他の利用者に害が無く、禁じる根拠が無い。

根拠の無い制限を足すと、後から「なぜ禁じたのか」が分からなくなる。

## 参考

- [基本設計 03 §4: いいね数の非正規化](../design/basic/03-database.md#4-いいね数の非正規化)
- [基本設計 05 §2: 交流](../design/basic/05-api.md#交流)
- [詳細設計 02: クラス設計](../design/detail/02-class-design.md)
- [api/internal/usecase/post.go](../../api/internal/usecase/post.go)（可視性の判定）
