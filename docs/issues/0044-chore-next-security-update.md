| 項目 | 値 |
| --- | --- |
| タイトル | `chore: Next.js の脆弱性に対応する` |
| ラベル | `bug` |
| マイルストーン | なし |
| 起票済み | [#97](https://github.com/yama-shu/575-sns/issues/97) |

---

## 背景・目的

`npm audit` が3件を検出し、CI の web が失敗している。

| パッケージ | 深刻度 | 内容 |
| --- | --- | --- |
| `next` 16.0.0 〜 16.3.2 | **critical** | 画像最適化 API で AVIF を扱う際の未認証リモートコード実行（[GHSA-2xp9-vwfh-vxw4](https://github.com/advisories/GHSA-2xp9-vwfh-vxw4)）、Windows で動かす場合の未認証リモートコード実行（[GHSA-p293-qw3h-jr36](https://github.com/advisories/GHSA-p293-qw3h-jr36)） |
| `sharp` < 0.35.4 | high | libheif の脆弱性（[GHSA-rgj7-g3m4-5g8c](https://github.com/advisories/GHSA-rgj7-g3m4-5g8c)） |
| `js-yaml` | high | `eslint-config-next` の推移的依存 |

**新しく公表されたものであり、変更によって持ち込んだものではない。**

## 影響の範囲

**Windows の件は該当しない。** 本番は Linux で動いている。

**画像最適化 API の件は該当しうる。** `next/image` をどこでも使っていないが、
`/_next/image` の経路は既定で有効であり、公開中のサーバーで応答する
（404 ではなく 400 を返す）。

## やること

- [x] `next` と `eslint-config-next` を 16.3.5 にする
- [x] `sharp` と `js-yaml` を更新する

## 完了条件

- [x] `npm audit --audit-level=high` が 0 件
- [x] `npm ci` が通る（ロックファイルの整合）
- [x] lint・型検査・ビルドが通る
- [x] **E2E が通る**（16.3.0 からの更新で挙動が変わっていないこと）

## やらないこと

- 画像最適化 API の無効化（更新で解消するため不要）
- `next/image` の導入

## 実装上の注意

### ロックファイルは Linux で生成する

macOS で `npm install` を実行すると、**Linux 専用の optional 依存が
ロックファイルから削除され、CI の `npm ci` が失敗する**（[#88](https://github.com/yama-shu/575-sns/issues/88) で発生）。

CI と同じ `node:24-bookworm-slim` のコンテナで生成する。

### 版を固定したまま上げる

`package.json` は `next` を `16.3.0` と完全一致で指定している。
`npm audit fix --force` に任せず、**16.3.5 を明示して上げる**。
`eslint-config-next` も同じ版に揃える。

## 参考

- [GHSA-2xp9-vwfh-vxw4](https://github.com/advisories/GHSA-2xp9-vwfh-vxw4)
- [GHSA-rgj7-g3m4-5g8c](https://github.com/advisories/GHSA-rgj7-g3m4-5g8c)
