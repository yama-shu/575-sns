| 項目 | 値 |
| --- | --- |
| タイトル | `feat: 投稿詳細とユーザーページを実装する` |
| ラベル | `feature` |
| マイルストーン | M4 画面 |
| 起票済み | [#60](https://github.com/yama-shu/575-sns/issues/60) |

---

## 背景・目的

[S-03 投稿詳細](../design/basic/04-screens.md#1-画面一覧)（[FR-03-04](../requirements/01-requirements.md#fr-03-閲覧)）と
[S-04 ユーザーページ](../design/basic/04-screens.md#1-画面一覧)（[FR-03-05](../requirements/01-requirements.md#fr-03-閲覧)）を実装する。

**いま画面からは誰もフォローできない。** フォロー中タイムライン（S-02）という画面がありながら、
フォローするには `curl` を叩くしかない。いいねも数が出るだけで押せない。
自分の投稿を消す導線も無い。

api は揃っている。[#58](https://github.com/yama-shu/575-sns/issues/58) でプロフィールと投稿一覧を実装し、
いいね・フォロー・ブロック・削除は [#30](https://github.com/yama-shu/575-sns/issues/30) [#34](https://github.com/yama-shu/575-sns/issues/34) [#36](https://github.com/yama-shu/575-sns/issues/36) [#43](https://github.com/yama-shu/575-sns/issues/43) で実装済みである。
**この Issue で、画面から使えるようになる。**

## やること

- [x] `/@:handle` へ行けるようにする（[下記](#handle-の経路)）
- [x] S-03 投稿詳細を実装する
- [x] S-04 ユーザーページを実装する
- [x] いいねの操作を実装する
- [x] フォロー・解除を実装する
- [x] ブロック・解除を実装する
- [x] 投稿の削除を実装する
- [x] 投稿カードから両画面へ行けるようにする
- [x] README を更新する

## 完了条件

### 投稿詳細（S-03）

- [x] `/posts/:id` で1件が見える
- [x] 句ごとに改行されている
- [x] 投稿者・判定・いいね数・日時が出る
- [x] 未ログインでも閲覧できる
- [x] 見えない投稿は 404 の画面になる（削除済み・ブロック・フォロワー限定）
- [x] 投稿者名からユーザーページへ行ける

### ユーザーページ（S-04）

- [x] `/@:handle` で見える
- [x] 表示名・識別名・自己紹介・登録日が出る
- [x] 投稿数・フォロー数・フォロワー数が出る
- [x] その人の投稿が新しい順に並び、**遡れる**
- [x] 未ログインでも閲覧できる
- [x] 見えない相手は 404 の画面になる
- [x] 投稿が0件のとき、その旨が分かる

### いいね（[FR-04-01](../requirements/01-requirements.md#fr-04-交流)）

- [x] 押すといいねが付き、数が増える
- [x] もう一度押すと取り消され、数が減る
- [x] 連打しても数がずれない
- [x] 未ログインでは押せない（ログインへ誘導する）
- [x] **JavaScript が無効でも押せる**（[下記](#操作はすべてフォームにする)）

### フォロー（[FR-04-02](../requirements/01-requirements.md#fr-04-交流)）

- [x] フォロー・解除ができ、フォロワー数が変わる
- [x] 自分のページには出ない
- [x] 未ログインでは出ない（ログインへ誘導する）
- [x] フォローするとフォロー中タイムラインに出るようになる

### ブロック（[FR-05-02](../requirements/01-requirements.md#fr-05-健全性)）

- [x] ブロック・解除ができる
- [x] ブロック中はその旨が分かり、投稿が0件になる
- [x] **ブロックすると双方向にフォローが外れる**（BR-08）
- [x] 解除できる（[#58](https://github.com/yama-shu/575-sns/issues/58) がそのために 404 にしていない）

### 削除（[FR-02-07](../requirements/01-requirements.md#fr-02-投稿詠む)）

- [x] 自分の投稿にだけ出る
- [x] **一度で消えない**（[下記](#削除は二段にする)）
- [x] 消すと一覧から消える
- [x] 他人の投稿は消せない

### 共通

- [x] `npm run lint` `npm run typecheck` `npm run build` が通る

## やらないこと

- 通報のモーダル（S-12。別 Issue）
- フォロー中一覧・フォロワー一覧（S-05 / S-06。別 Issue）
- プロフィール編集（S-10。別 Issue）
- 返歌（[FR-02-10](../requirements/01-requirements.md#fr-02-投稿詠む) は Could）
- E2E テスト（画面が揃ってから）

## 実装上の注意

### `@handle` の経路

[基本設計 04 §1](../design/basic/04-screens.md#1-画面一覧) はユーザーページを `/@:handle` としている。

**App Router で `@` 始まりのディレクトリは使えない。** `app/@[handle]/` は
並列ルートのスロットとして解釈される。

`app/users/[handle]/` を置き、`next.config.ts` の rewrite で
`/@:handle` を割り当てる。**先に試した。**

```
/@tarou        → 200・描画は 'HANDLE_IS_<!-- -->tarou'（@ は付かない）
/@tarou        → リダイレクトではない（redirect_url は空）
/@tarou/extra  → 404
/@             → 404
/ /login       → 200（既存の経路は壊れない）
```

`app/[handle]/` を根に置く案は採らない。静的な経路が優先されるとはいえ、
**あらゆる未知のパスがユーザーページに吸われる**形になり、
新しい画面を足すたびに衝突を気にすることになる。

### 操作はすべてフォームにする

いいね・フォロー・ブロック・削除を `<button onClick>` で書かない。
`<form action={サーバーアクション}>` にする。

[NFR-06-03](../requirements/01-requirements.md#nfr-06-ユーザビリティ)（主要な操作はキーボードのみで完結）を満たし、
**JavaScript が無効でも動く**。[#50](https://github.com/yama-shu/575-sns/issues/50) の「もっと読む」、
[#52](https://github.com/yama-shu/575-sns/issues/52) の投稿と同じ方針である。

JavaScript があるときは画面遷移せずに結果だけ差し替える。

### 削除は二段にする

一度の操作で消せると誤って消す。投稿は編集できない（[FR-02-07](../requirements/01-requirements.md#fr-02-投稿詠む)）ため、
取り返しがつかない。

**`confirm()` に頼らない。** JavaScript が無効な環境では確認が出ないまま消える。
`<details>` で畳んでおき、開いてから押す形にする。

### 連打で数がずれないこと

いいねは冪等（PUT / DELETE）であり、api は `like_count` を返す。
**画面は返ってきた数をそのまま表示する。** 自分で足し引きすると、
連打や通信の失敗でずれる。

### 見えない相手・見えない投稿は 404 の画面にする

api が 404 を返す条件は [#58](https://github.com/yama-shu/575-sns/issues/58) と [#30](https://github.com/yama-shu/575-sns/issues/30) で決まっている。
**画面は理由を出し分けない。** 「ブロックされています」と出すと BR-10 に反する。

`notFound()` を呼び、共通の404画面に落とす。

### 投稿一覧の部品を使い回す

ユーザーページの投稿一覧は [#50](https://github.com/yama-shu/575-sns/issues/50) の `Timeline` を使う。
`kind` に利用者ぶんを足し、続きの取得先を切り替える。

**別の部品を作らない。** 無限スクロールと「もっと読む」の両立を2箇所に書くと、
片方だけ直したときに食い違う。

## 参考

- [基本設計 04 §1: 画面一覧](../design/basic/04-screens.md#1-画面一覧)
- [基本設計 04 §2: 画面遷移図](../design/basic/04-screens.md#2-画面遷移図)
- [基本設計 02: 関係に関するルール](../design/basic/02-domain-model.md)
- [api/openapi.yaml](../../api/openapi.yaml)
