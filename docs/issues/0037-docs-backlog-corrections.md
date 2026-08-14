| 項目 | 値 |
| --- | --- |
| タイトル | `docs: バックログの記述を実態に合わせる` |
| ラベル | `documentation` |
| マイルストーン | なし |
| 起票済み | [#82](https://github.com/yama-shu/575-sns/issues/82) |

---

## 背景・目的

[docs/issues/README.md](README.md) のバックログに、実態と食い違う記述が2件ある。
**バックログは次の着手先を決める資料であり、誤ったまま残すと判断を誤らせる。**

## 1. 暫定公開の構成が既存サーバーと競合する

M5 第1期に次の記述があった。

> `infra` | docker compose + Caddy で暫定公開し、Let's Encrypt で HTTPS 化する

**既存の VPS では nginx が 443 を使用している。** oil_game のリバースプロキシとして稼働しており、
証明書も certbot（`authenticator = nginx` / `installer = nginx`）で取得し、
`certbot.timer` が自動更新している。

ここに Caddy を追加すると 443 が競合し、[ADR-0007](../adr/0007-hosting-conoha-vps.md) の制約 D
「oil_game を巻き込んで落とさないこと」に反する。

**既存の nginx をリバースプロキシとして使い、certbot でサブドメインの証明書を追加する**形に改める。
既存の更新の仕組みにそのまま乗るため、新たに導入するものが無い。

## 2. Should の一覧から利用停止が抜けている

「Should（Must がすべて終わってから）」の表に、退会（FR-01-04）と
パスワード再設定（FR-01-05）しか無かった。

**利用停止（[FR-05-04](../requirements/01-requirements.md#fr-05-健全性)）が抜けている。**
[#74](https://github.com/yama-shu/575-sns/issues/74) と [#76](https://github.com/yama-shu/575-sns/issues/76) が
「やらないこと」で「別 Issue」としたまま、バックログに載せていなかった。

## やること

- [x] 第1期の暫定公開の記述を nginx + certbot に改める
- [x] Should の表に利用停止を足す

## 完了条件

- [x] バックログの記述が実態と一致する
- [x] 変更の理由が追える（本ドラフトと [#82](https://github.com/yama-shu/575-sns/issues/82) から辿れる）

## やらないこと

- 暫定公開の実施（記述を直すだけ）
- 利用停止の実装・ドラフト作成（着手が近づいた時点で行う）
