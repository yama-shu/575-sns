| 項目 | 値 |
| --- | --- |
| タイトル | `feat: フォロー中・フォロワー・ブロック中の一覧を返す API を実装する` |
| ラベル | `feature` |
| マイルストーン | M4 画面 |
| 起票済み | [#68](https://github.com/yama-shu/575-sns/issues/68) |

---

## 背景・目的

[S-05 フォロー中一覧](../design/basic/04-screens.md#1-画面一覧) /
[S-06 フォロワー一覧](../design/basic/04-screens.md#1-画面一覧)（[FR-04-03](../requirements/01-requirements.md#fr-04-交流)）と
[S-11 ブロック中一覧](../design/basic/04-screens.md#1-画面一覧)（[FR-05-02](../requirements/01-requirements.md#fr-05-健全性)）を作るための API を実装する。

[基本設計 05](../design/basic/05-api.md) は3本を定めているが、いずれも未実装である。

| メソッド | パス | 認証 |
| --- | --- | :---: |
| GET | `/api/v1/users/:handle/following` | — |
| GET | `/api/v1/users/:handle/followers` | — |
| GET | `/api/v1/me/blocks` | ✅ |

**画面と分ける。** [#58](https://github.com/yama-shu/575-sns/issues/58) と同じく、経路が3本あり可視性の規則も伴うため、
1つの Issue にすると変更が大きくなる。

## やること

- [ ] 3本の経路を実装する
- [ ] 一覧の項目を決める（[下記](#一覧に何を返すか)）
- [ ] カーソルの基準を決める（[下記](#カーソルの基準)）
- [ ] 実行計画を実データ量で測る（[下記](#実行計画は実データ量で測る)）
- [ ] `openapi.yaml` に追記し、適合テストを通す
- [ ] web の型を生成し直す
- [ ] README を更新する

## 完了条件

### 一覧が返る

- [ ] フォロー中一覧が返る
- [ ] フォロワー一覧が返る
- [ ] ブロック中一覧が返る
- [ ] 識別名・表示名・自己紹介が返る
- [ ] ログイン済みなら**その相手をフォローしているか**が返る
- [ ] カーソルで遡れる（`next_cursor`）
- [ ] 件数の上限が効く（1〜50）

### 見える範囲

- [ ] フォロー中・フォロワーは未ログインでも見られる
- [ ] ブロック中一覧は**本人だけ**（未ログインは 401）
- [ ] 対象の利用者が見えないときは 404（[#58](https://github.com/yama-shu/575-sns/issues/58) と同じ扱い）
- [ ] 一覧の中身から、**閲覧者を見られない相手を除く**（[下記](#一覧から誰を外すか)）
- [ ] 利用停止・退会した利用者を一覧に含めない

### 共通

- [ ] `additionalProperties: false` の適合テストが通る
- [ ] `./scripts/check.sh api` が通る
- [ ] カバレッジ目標を下回らない（[詳細設計 04](../design/detail/04-test-design.md) の usecase 90% / infra 70%）

## やらないこと

- 画面（S-05 / S-06 / S-11。次の Issue）
- いいねした人の一覧（設計に無い）
- 一覧の並べ替え（新しい順の1種類だけ）

## 実装上の注意

### 一覧に何を返すか

**プロフィールをそのまま返さない。** `post_count` などの数え上げが人数ぶん走る。

一覧に出すのは識別名・表示名・自己紹介・アイコン URL と、
閲覧者から見た `following` だけにする。

`following` は投稿一覧の `liked_by_me` と同じく **EXISTS で1回のクエリにまとめる**。
1人ずつ問い合わせると、50人の一覧で 51 回のクエリになる。

### カーソルの基準

`follows` と `blocks` に `id` 列は無い。主キーは
`(follower_id, followee_id)` と `(blocker_id, blocked_id)` である。

**相手の利用者 ID を基準にする。** `created_at` は同時刻の並びが定まらず、
カーソルの基準にすると取りこぼしうる（[基本設計 03 §5](../design/basic/03-database.md#5-カーソルページネーション) が
同じ理由で `id` を使っている）。

### 一覧から誰を外すか

[#58](https://github.com/yama-shu/575-sns/issues/58) で「相手が閲覧者をブロックしていると 404」とした。
**一覧にも同じ規則を適用する。**

適用しないと、一覧に並ぶが開くと 404 になる相手が出る。

- 閲覧者をブロックしている相手を外す
- 利用停止・退会した相手を外す

自分がブロックしている相手は外さない（[#58](https://github.com/yama-shu/575-sns/issues/58) でプロフィールは見えるようにしたため）。
そもそも BR-08 によりフォロー関係は残らないため、フォロー系の一覧には出ない。

### 実行計画は実データ量で測る

数行しかないテーブルでは PostgreSQL が正しく Seq Scan を選ぶ。
**いまの手元のデータでは判断できない**（[#41](https://github.com/yama-shu/575-sns/issues/41) で同じ誤りをした）。

行を投入したうえで `EXPLAIN` を見る。既存のインデックスは次の2つである。

```
follows_pkey             (follower_id, followee_id)
follows_followee_id_idx  (followee_id)
blocks_pkey              (blocker_id, blocked_id)
```

フォロワー一覧は `followee_id` で絞って `follower_id` の順に並べる。
**索引が `followee_id` だけのため、並べ替えが要るかもしれない。**
計画を見て、必要ならインデックスを追加する。追加する場合はマイグレーションを足す。

## 参考

- [基本設計 05 §ユーザー・交流・健全性](../design/basic/05-api.md)
- [基本設計 03 §5: カーソルページネーション](../design/basic/03-database.md#5-カーソルページネーション)
- [api/openapi.yaml](../../api/openapi.yaml)
