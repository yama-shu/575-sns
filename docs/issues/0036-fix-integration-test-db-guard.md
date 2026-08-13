| 項目 | 値 |
| --- | --- |
| タイトル | `fix: 結合テストが開発用のデータベースを消すのを防ぐ` |
| ラベル | `bug` |
| マイルストーン | なし |
| 起票済み | [#80](https://github.com/yama-shu/575-sns/issues/80) |

---

## 背景・目的

**結合テストを実行すると、開発用のデータが消える。**

[`api/internal/infra/postgres/integration_test.go`](../../api/internal/infra/postgres/integration_test.go) は
各テストの前後で `reports` / `posts` / `users` を全件削除する。
接続先は `API_DATABASE_URL` であり、**api のコンテナではこれが開発用の DB を指す**。

そのうえ、同ファイルの先頭のコメントと [README](../../README.md) が、その実行方法を勧めている。

```
//	docker compose exec api go test ./internal/infra/postgres/... -v
```

```bash
# api — go test（結合テストは API_DATABASE_URL があるときだけ走る）
docker compose exec api go test ./... -cover
```

**実際にこれを実行して、手元の利用者・投稿がすべて消えた。**
消えたのは動作確認用のデータであり、作り直して復旧した。

## やること

- [ ] 接続先を `API_DATABASE_URL` とは別の環境変数で受ける
- [ ] 接続先が**テスト用の DB であることを確かめる**（[下記](#db-名も確かめる)）
- [ ] テスト用の DB を用意する手順を作る
- [ ] CI を新しい環境変数に合わせる
- [ ] README と `integration_test.go` の実行方法を直す
- [ ] 詳細設計 04 §7 に実行の前提を書く

## 完了条件

- [ ] 開発用の DSN を渡すと、**テストが実行される前に失敗する**
- [ ] そのとき**開発用のデータが消えていない**
- [ ] テスト用の DSN を渡すと、結合テストがすべて通る
- [ ] 環境変数が未設定なら、これまでどおりスキップする
- [ ] CI で結合テストが動き続けている（スキップされていないこと）
- [ ] `./scripts/check.sh` が通る

## やらないこと

- 結合テストの中身の変更（消す対象・テストケースは変えない）
- テスト用 DB の自動生成をテストコードに入れること（[下記](#db-の作成をテストにやらせない)）
- `cleanup()` を `TRUNCATE` にするなどの高速化

## 実装上の注意

### 環境変数を分ける

`API_DATABASE_URL` は **api 本体・マイグレーション・セッション削除が使う変数**である。
これを結合テストと共有している限り、api が動く場所で `go test` を叩けば消える。

結合テストは `API_TEST_DATABASE_URL` から読む。未設定ならスキップする。

### DB 名も確かめる

環境変数を分けただけでは、**その変数に開発用の DSN を書いた場合を防げない**。

接続してから `SELECT current_database()` を引き、
名前が `_test` で終わらなければ失敗させる。
「消してよい DB か」を接続先そのものに確かめる。

### DB の作成をテストにやらせない

テストが `CREATE DATABASE` を実行すると、**そのために管理者権限の接続が要る**。
消してよい DB を用意するのは、テストの外の作業にする。

`scripts/testdb.sh` で作成とマイグレーションを行う。

### CI では作り分ける

CI の PostgreSQL は使い捨てだが、**同じ規則を通す**。
DB 名を `sns575_test` にし、マイグレーションもテストもそこへ向ける。

規則を CI だけ免除すると、手元と CI で通る条件が変わる。

## 参考

- [詳細設計 04 §7: api のテスト設計](../design/detail/04-test-design.md#7-api-のテスト設計)
- [基本設計 03: データベース設計](../design/basic/03-database.md)
