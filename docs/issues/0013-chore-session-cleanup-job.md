| 項目 | 値 |
| --- | --- |
| タイトル | `chore: 期限切れセッションを削除する定期ジョブを実装する` |
| ラベル | `chore` |
| マイルストーン | M2 API 基盤 |
| 起票済み | [#32](https://github.com/yama-shu/575-sns/issues/32) |

---

## 背景・目的

[ADR-0006](../adr/0006-authentication.md) が、サーバー側セッションを選んだ代償として
定期的な削除を要求している。

> セッションテーブルが際限なく増える。定期ジョブで期限切れを削除する

放置すると、ログインのたびに増える行が一度も減らない。
`sessions` の主キー検索は認証のたびに走るため、行数の増加は認証の速度に効く。

[#25](https://github.com/yama-shu/575-sns/issues/25) で
`usecase.Auth.DeleteExpiredSessions` と `SessionRepository.DeleteExpired` は実装済みで、
**呼び出す仕組みだけが無い**。

## やること

- [ ] `cmd/cleanup` を実装する（[下記](#api-の中でタイマーを回さない)）
- [ ] 削除件数を構造化ログに出す
- [ ] compose に `cleanup` サービスを追加する
- [ ] 単体テストを書く
- [ ] README に実行方法を追記する

## 完了条件

- [ ] 期限切れのセッションだけが削除される（有効なセッションは残る）
- [ ] 削除件数が構造化ログに出る（`event` で集計できる）
- [ ] 対象が0件でも正常終了する（終了コード 0）
- [ ] DB へ接続できないとき終了コード 1 で終わる（Kubernetes の Job が失敗を検知できる）
- [ ] 同時に2つ動かしてもエラーにならない
- [ ] `docker compose run --rm cleanup` で実行できる
- [ ] ログにセッション ID が出ていない（[詳細設計 03](../design/detail/03-error-handling.md#ログに出してはいけないもの)）
- [ ] `gofmt` の差分がなく、`golangci-lint` が 0 issues

## やらないこと

- Kubernetes の CronJob マニフェスト（M5。マニフェスト作成の Issue でまとめて扱う）
- 論理削除した投稿の物理削除（M5 のバックログ）
- 削除の分割実行（[下記](#分割して削除しない)）
- compose での自動定期実行（[下記](#compose-では自動起動しない)）

## 実装上の注意

### api の中でタイマーを回さない

api の中で `time.Ticker` を回すと、**Pod の数だけ同じ削除が走る**。
本番では api が複数 Pod にスケールする（[NFR-03-01](../requirements/01-requirements.md#nfr-03-拡張性)）。

削除自体は冪等なので壊れはしないが、同じ行を狙う DELETE が同時に走り、
不要なロック競合を生む。何より「1つのジョブが定期的に走る」という意図が
Pod 数に依存して崩れる。

[マイグレーションと同じ構成](../design/basic/03-database.md#6-マイグレーション方針)にする。

| | 実行単位 | 本番での実行 |
| --- | --- | --- |
| `migrate` | 独立したバイナリ | Kubernetes の Job |
| `cleanup` | 独立したバイナリ | Kubernetes の CronJob |

api と同じイメージに入れ、`command` だけ差し替える。
イメージを分けると、api とジョブのバージョンがずれた組み合わせでデプロイできてしまう。

### 1回実行して終了する

常駐せず、実行して終了する。CronJob の実行単位と一致させるためである。

常駐プロセスにすると、次の実行時刻を持つのが**アプリケーションと Kubernetes の両方**になり、
どちらの設定が効いているのか分からなくなる。

### 分割して削除しない

`DELETE FROM sessions WHERE expires_at < now()` を1回で実行する。

大量の行を1回で消すとロックが長引くが、575 の想定規模
（同時接続 1,000・[NFR-01-03](../requirements/01-requirements.md#nfr-01-性能)）では
1日あたりの期限切れは多くとも数千行である。分割の複雑さに見合わない。

規模が変わったら分割する。判断の材料は実測であり、
いま予防的に入れるものではない。

### compose では自動起動しない

`profiles` を使い、`docker compose up` では起動しないようにする。

ローカル開発でセッションが溜まっても困らない一方、
起動のたびに走ると「いつ消えたのか」が分かりにくくなる。
手で実行する。

### ログに出すもの・出さないもの

| 項目 | 出す |
| --- | :---: |
| 削除件数 | ✅ |
| 処理時間 | ✅ |
| セッション ID | ❌ |
| 利用者 ID | ❌ |

セッション ID はログイン状態そのものであり、ログに出すと
ログを読める者が他人になりすませる。

## 参考

- [ADR-0006: 認証方式](../adr/0006-authentication.md)
- [基本設計 03 §6: マイグレーション方針](../design/basic/03-database.md#6-マイグレーション方針)
- [詳細設計 03 §3: ログ設計](../design/detail/03-error-handling.md)
- [api/internal/usecase/auth.go](../../api/internal/usecase/auth.go)（`DeleteExpiredSessions`）
