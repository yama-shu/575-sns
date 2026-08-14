| 項目 | 値 |
| --- | --- |
| タイトル | `chore: 依存の脆弱性検査の失敗を解消する` |
| ラベル | `chore` |
| マイルストーン | なし |
| 起票済み | [#88](https://github.com/yama-shu/575-sns/issues/88) |

---

## 背景・目的

CI の依存の脆弱性検査が2件失敗している。

| ジョブ | 検査 | 内容 |
| --- | --- | --- |
| api | `govulncheck` | Go 標準ライブラリの脆弱性6件。`go1.26.5` で検出、`go1.26.6` で修正済み |
| web | `npm audit` | `nanoid < 3.3.18`（high）。`next` → `postcss` の推移的依存 |

**新しく公表された脆弱性であり、変更によって持ち込んだものではない。**

直近の `main` が成功していたのは、[#84](https://github.com/yama-shu/575-sns/issues/84) の変更が
docs のみで api / web のジョブがスキップされていたためである。
`.github/workflows/ci.yml` を変更する [#86](https://github.com/yama-shu/575-sns/issues/86) で
全サービスが実行され、表面化した。

## やること

- [ ] api が最新のパッチ版の Go を使うようにする
- [ ] web の `nanoid` を更新する

## 完了条件

- [ ] `govulncheck` が通る
- [ ] `npm audit --audit-level=high` が通る
- [ ] `./scripts/check.sh` が通る

## やらないこと

- 依存の一括更新（この2件のみを対象とする）
- Dependabot 等の自動更新の導入（別途検討）

## 実装上の注意

### Go はパッチ版を指定しない

`api/go.mod` は `go 1.26` としか書いていない。`actions/setup-go` は指定が無いと
**ランナーに同梱された版**を使い、それが古いことがある（今回は `go1.26.5`）。

`check-latest: true` を付け、**常に最新のパッチ版を取りに行く**ようにする。

`go.mod` に `toolchain go1.26.6` と書いて固定する方法もあるが、採らない。
標準ライブラリの脆弱性が公表されるたびに手で書き換えることになり、
**修正が出ているのに落ち続ける状態**を繰り返す。

### nanoid は推移的依存

直接依存していない（`next` → `postcss` → `nanoid`）。
`npm audit fix` が `package-lock.json` を更新する。`package.json` は変わらない。

## 参考

- [GHSA-2v37-7h3g-55p8](https://github.com/advisories/GHSA-2v37-7h3g-55p8)（nanoid）
