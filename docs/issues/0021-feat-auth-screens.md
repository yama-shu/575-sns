| 項目 | 値 |
| --- | --- |
| タイトル | `feat: ログイン・登録画面を実装する` |
| ラベル | `feature` |
| マイルストーン | M4 画面 |
| 起票済み | [#48](https://github.com/yama-shu/575-sns/issues/48) |

---

## 背景・目的

[S-08 ログイン](../design/basic/04-screens.md#1-画面一覧)（[FR-01-02](../requirements/01-requirements.md#fr-01-アカウント)）と
[S-09 アカウント登録](../design/basic/04-screens.md#1-画面一覧)（[FR-01-01](../requirements/01-requirements.md#fr-01-アカウント)）を実装する。

M4 の最初の画面。**他のすべての画面がログイン状態を前提とする**ため、
[画面遷移図](../design/basic/04-screens.md#2-画面遷移図)でも他画面からの入口になっている。

api は [#25](https://github.com/yama-shu/575-sns/issues/25) で実装済み、型は [#46](https://github.com/yama-shu/575-sns/issues/46) で生成済みである。
web には画面が無く、疎通確認の暫定ページだけがある。

## やること

- [ ] api を呼ぶ土台を作る（[下記](#ブラウザから-api-を直接呼ばない)）
- [ ] エラーコードを利用者向けの文言に対応付ける
- [ ] 画面の骨格（レイアウト・ナビゲーション）を作る
- [ ] S-08 ログイン画面を実装する
- [ ] S-09 アカウント登録画面を実装する
- [ ] ログアウトを実装する
- [ ] ログイン後の遷移先（`/home`）を最小限で用意する（[下記](#home-は最小限にとどめる)）
- [ ] 未ログインで要ログイン画面を開いたときの扱いを実装する
- [ ] README に画面の確認方法を追記する

## 完了条件

### 登録（S-09）

- [ ] `/signup` で識別名・メール・パスワード・表示名を入力して登録できる
- [ ] 登録に成功すると**ログイン済みの状態で** `/home` へ移動する
- [ ] 識別名が使用済みのとき、その旨が画面に出る（`HANDLE_TAKEN`）
- [ ] メールアドレスが使用済みのとき、その旨が画面に出る（`EMAIL_TAKEN`）
- [ ] 入力の不備（形式・長さ）が項目ごとに分かる形で出る

### ログイン（S-08）

- [ ] `/login` で識別名とパスワードを入力してログインできる
- [ ] 成功すると `/home` へ移動する
- [ ] 失敗したとき「識別名またはパスワードが違います」と出る
- [ ] **どちらが違うかを表示しない**（[ADR-0006](../adr/0006-authentication.md)）
- [ ] 利用停止中のとき、その旨が出る（`ACCOUNT_SUSPENDED`）

### 共通

- [ ] ログアウトするとセッションが切れ、`/login` へ移動する
- [ ] ログイン済みで `/login` `/signup` を開くと `/home` へ移動する
- [ ] 未ログインで `/home` を開くと `/login` へ移動する
- [ ] **セッション Cookie がブラウザから読めない**（`HttpOnly` が効いている）
- [ ] 主要な操作がキーボードだけで完結する（[NFR-06-03](../requirements/01-requirements.md#nfr-06-ユーザビリティ)）
- [ ] 768px 以下で単一カラムになる（[NFR-06-01](../requirements/01-requirements.md#nfr-06-ユーザビリティ)）
- [ ] **エラーが「エラーが発生しました」だけにならない**（[NFR-06-02](../requirements/01-requirements.md#nfr-06-ユーザビリティ)）
- [ ] `npm run lint` `npm run typecheck` `npm run build` が通る

## やらないこと

- タイムラインの中身（次の Issue）。`/home` は遷移先として最小限にとどめる
- パスワード再設定（FR-01-05・Should）
- プロフィール編集（S-10。別 Issue）
- ソーシャルログイン（[MVP 後に再検討](../requirements/01-requirements.md)）
- E2E テスト（画面が揃ってから）

## 実装上の注意

### ブラウザから api を直接呼ばない

[基本設計 01 §6](../design/basic/01-architecture.md#6-サービス間通信) は
`web → api` を「利用者の Cookie を転送」と定めている。
**ブラウザは web だけと通信し、web が api へ中継する。**

ブラウザから api を直接呼ぶと、開発環境（`localhost:3000` → `localhost:8080`）が
別オリジンになり CORS の設定が要る。本番は単一オリジンの構成であり、
開発だけのために api へ CORS を足すと、本番では不要な設定が残る。

中継は Next.js のサーバー側（Server Actions / Route Handlers）で行う。

### `Set-Cookie` を中継する

ログイン・登録・ログアウトでは api が `Set-Cookie` を返す。
web はこれをブラウザへ引き渡す必要がある。

**`HttpOnly` を落とさない。** 落とすと JavaScript からセッション ID を読めてしまい、
[ADR-0006](../adr/0006-authentication.md) が Cookie を選んだ理由（XSS でセッションを盗まれない）が消える。

### エラーコードを文言に対応付ける

[基本設計 05](../design/basic/05-api.md#エラーレスポンス) のとおり **`code` で分岐する。**
`message` は文言が変わりうるため、これで分岐すると壊れる。

[NFR-06-02](../requirements/01-requirements.md#nfr-06-ユーザビリティ) が
「単に『エラー』とだけ表示しない」ことを求めている。
対応表を1箇所に置き、知らない `code` が来たときの既定の文言も決める。

| `code` | 画面での扱い |
| --- | --- |
| `VALIDATION_FAILED` | `details.field` の項目に紐づけて表示する |
| `HANDLE_TAKEN` / `EMAIL_TAKEN` | 該当する項目に紐づけて表示する |
| `INVALID_CREDENTIALS` | フォーム全体のエラーとして表示する |
| `ACCOUNT_SUSPENDED` | フォーム全体のエラーとして表示する |
| 知らない `code` | 「エラーが発生しました」＋ `request_id` |

### 生成した型を使う

[#46](https://github.com/yama-shu/575-sns/issues/46) で生成した `web/src/lib/api/schema.d.ts` を使う。
**手で型を書かない。** 書くと api の変更に追随できず、生成した意味が無くなる。

### `/home` は最小限にとどめる

[画面遷移図](../design/basic/04-screens.md#2-画面遷移図)はログイン成功後の遷移先を
S-02（フォロー中タイムライン `/home`）としている。

タイムラインは次の Issue で実装するため、本 Issue では
**ログイン済みであることが分かる最小限の画面**を置く。
遷移先を `/` に変えると設計と食い違い、次の Issue で戻すことになる。

### 見た目の作り方

CSS Modules（Next.js に組み込み）と1つのグローバルスタイルで作る。

**CSS フレームワークを入れない。** 画面は 13 枚、要素は入力欄・ボタン・
投稿カードが中心であり、フレームワークの学習と設定に見合う規模ではない。
[ADR-0002](../adr/0002-tech-stack.md) が依存を増やす判断に根拠を求めているのと同じ考え方である。

必要になったら入れる。判断の材料は実際の画面である。

### キーボードだけで完結させる

[NFR-06-03](../requirements/01-requirements.md#nfr-06-ユーザビリティ)。
`<form>` と `<button type="submit">` を使い、Enter で送信できるようにする。
`<div onClick>` のような、キーボードで到達できない作りにしない。

エラーは `aria-describedby` で入力欄に紐づけ、読み上げでも対応が分かるようにする。

## 参考

- [基本設計 04: 画面設計](../design/basic/04-screens.md)
- [基本設計 01 §6: サービス間通信](../design/basic/01-architecture.md#6-サービス間通信)
- [基本設計 05 §1: エラーレスポンス](../design/basic/05-api.md#エラーレスポンス)
- [ADR-0006: 認証方式](../adr/0006-authentication.md)
- [api/openapi.yaml](../../api/openapi.yaml)
