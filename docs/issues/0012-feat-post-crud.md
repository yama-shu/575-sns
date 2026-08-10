| 項目 | 値 |
| --- | --- |
| タイトル | `feat: 投稿の作成・取得・削除を実装する` |
| ラベル | `feature` |
| マイルストーン | M2 API 基盤 |
| 起票済み | [#30](https://github.com/yama-shu/575-sns/issues/30) |

---

## 背景・目的

五七五を保存できるようにする。575 の中心機能であり、
タイムライン・いいね・フォローのすべてがこれを前提とする。

材料は揃っている。

| 依存 | 状態 |
| --- | --- |
| `posts` テーブルと制約 | [#20](https://github.com/yama-shu/575-sns/issues/20) で作成済み |
| 認証（投稿者の特定） | [#25](https://github.com/yama-shu/575-sns/issues/25) で実装済み |
| 判定エンジンの呼び出し | [#28](https://github.com/yama-shu/575-sns/issues/28) で実装済み |

## やること

- [ ] `domain` に `Post` と `PostRepository` を定義する
- [ ] 判定結果から保存する値を組み立てる（[下記](#判定結果を投稿へ変換する)）
- [ ] `infra/postgres` に `PostRepository` を実装する
- [ ] `usecase` に作成・取得・削除を実装する
- [ ] エラーコード `PROSODY_HACHO` / `PROSODY_UNKNOWN_READING` を追加する
- [ ] エンドポイントを実装する
  - [ ] `POST /api/v1/posts`
  - [ ] `GET  /api/v1/posts/:id`
  - [ ] `DELETE /api/v1/posts/:id`
- [ ] usecase の単体テストを書く（リポジトリと判定エンジンをモックする）
- [ ] DB を使った結合テストを書く
- [ ] 本文の上限を 100 文字に揃える（[下記](#本文の上限が設計と食い違っている)）

## 完了条件

- [ ] 定型・許容の本文が 201 で保存され、`GET` で同じ内容が返る
- [ ] 破調が 422 `PROSODY_HACHO` で拒否され、**保存されない**
- [ ] 読めない語を含む本文が 422 `PROSODY_UNKNOWN_READING` で拒否される
- [ ] クライアントが判定結果を添えても無視され、サーバー側の再判定が使われる
- [ ] 保存される本文が prosody の `normalized_text` である（[下記](#保存するのは正規化後の本文)）
- [ ] `break1` / `break2` で本文を3句に復元できる
- [ ] 101 文字の本文が 400 `VALIDATION_FAILED` で拒否される
- [ ] 未ログインの投稿が 401 になる
- [ ] 他人の投稿の削除が 403 `FORBIDDEN` になる（[BR-03](../design/basic/02-domain-model.md#投稿に関するルール)）
- [ ] 削除した投稿の `GET` が 404 になる
- [ ] 削除済み投稿の再削除が 404 になる
- [ ] 存在しない ID・数値でない ID の `GET` が 404 / 400 になる
- [ ] `visibility=followers` の投稿が、フォローしていない利用者から見えない
- [ ] prosody 停止中に `POST` が 503 になり、`GET` は 200 のままである
- [ ] usecase の C1 カバレッジ 90% 以上（[NFR-05-04](../requirements/01-requirements.md#nfr-05-保守性運用性)）
- [ ] `gofmt` の差分がなく、`golangci-lint` が 0 issues

## やらないこと

- 投稿の編集（[BR-02](../design/basic/02-domain-model.md#投稿に関するルール)。仕様として存在しない）
- タイムライン取得（M3）
- いいね・フォロー・通報（M3）
- 画像の添付（要件に無い）
- レート制限（別 Issue。`POST /posts` は 20回/時/ユーザーの対象）

## 実装上の注意

### サーバー側で必ず再判定する

[基本設計 01 §4](../design/basic/01-architecture.md#なぜ2回判定するのか) のとおり、
入力中の判定は体験のためのものであり、信頼しない。

```
① 入力中（POST /prosody/check）  体験    信頼しない
② 保存前（POST /posts の内部）    正しさ   これが唯一の正
```

リクエストに `verdict` が含まれていても読まない。
読むと、`POST /api/v1/posts` に「判定OK」という嘘を添えるだけで破調が保存できる。

### 保存するのは正規化後の本文

prosody が返す `normalized_text` を `posts.body` に保存する。

区切り位置（`break1` / `break2`）はこの文字列上の位置である。
元の入力を保存すると、全角空白の圧縮などで位置がずれ、本文を3句に分けられなくなる
（[#8 の実装で判明](https://github.com/yama-shu/575-sns/pull/17)）。

### 判定結果を投稿へ変換する

`break1` / `break2` は句の文字数から求める。**バイト数ではなく文字数**で数える。

```
break1 = len([]rune(segments[0].Text))
break2 = break1 + len([]rune(segments[1].Text))
```

DB 制約 `0 < break1 AND break1 < break2 AND break2 < char_length(body)` を
満たすことは、句を連結すると `normalized_text` に戻る不変条件から導ける。

### 本文の上限が設計と食い違っている

[基本設計 03](../design/basic/03-database.md#body-の上限を100文字とする根拠) と
`posts.body VARCHAR(100)` は 100 文字だが、
[#28 で実装した `usecase.BodyMaxLength` は 140 文字](https://github.com/yama-shu/575-sns/pull/29)である。
Twitter の文字数を引きずった誤りで、設計にも要件にも根拠がない。

このままだと、101〜140 文字の本文が `POST /prosody/check` を通過し、
`POST /posts` で DB のエラーになる。100 文字に揃える。

### 削除は論理削除

`status = 'deleted'` と `deleted_at` を設定する。行は消さない。

DB 制約 `posts_deleted_at_consistency_check` が
「削除済みなら削除日時があり、そうでなければ無い」を保証するため、
片方だけ更新すると保存に失敗する。

削除できるのは投稿者本人だけ（[BR-03](../design/basic/02-domain-model.md#投稿に関するルール)）。
他人の投稿への削除は 403 とする。**404 にはしない。**
存在の有無を隠す必要があるのは非公開の投稿であり、
公開投稿の存在はすでに `GET` で分かるためである。

### `visibility=followers` の扱い

フォロー機能は M3 だが、`follows` テーブルは [#20](https://github.com/yama-shu/575-sns/issues/20) で作成済みである。
取得時にフォロー関係を確認し、フォローしていない利用者には 404 を返す。

「M3 まで誰でも見える」としないのは、**後から絞る実装は忘れられる**ためである。
いま参照する行が0件でも、条件を先に入れておけば漏れない。

### `liked_by_me`

[基本設計 05](../design/basic/05-api.md#post-apiv1posts) の応答に含まれる。
いいねは M3 だが `likes` テーブルは作成済みのため、問い合わせて返す。
常に `false` を返す実装にすると、M3 で直し忘れたときに気づけない。

## 参考

- [基本設計 01 §4: なぜ2回判定するのか](../design/basic/01-architecture.md#なぜ2回判定するのか)
- [基本設計 02 §5: 投稿に関するルール](../design/basic/02-domain-model.md#投稿に関するルール)
- [基本設計 03 §2: posts](../design/basic/03-database.md)
- [基本設計 05 §3: POST /api/v1/posts](../design/basic/05-api.md#post-apiv1posts)
- [詳細設計 02 §2: api のパッケージ構成](../design/detail/02-class-design.md#2-api-のパッケージ構成)
- [詳細設計 03 §2: エラー一覧](../design/detail/03-error-handling.md#2-エラー一覧)
