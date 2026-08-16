| 項目 | 値 |
| --- | --- |
| タイトル | `infra: 監視スタックのマニフェストを作成し、k3d で実証する` |
| ラベル | `infrastructure` |
| マイルストーン | M5 公開 |
| 起票済み | [#93](https://github.com/yama-shu/575-sns/issues/93) |

---

## 背景・目的

[ADR-0007](../adr/0007-hosting-conoha-vps.md) の構成図は Prometheus と Grafana を置いている。
[#91](https://github.com/yama-shu/575-sns/issues/91) でアプリのマニフェストを作ったので、監視も同じ形で用意する。

**本番へは適用しない。** ローカルの k3d で作り、実証するところまでとする。
VPS への適用はメモリの実測を経てから判断する（[下記](#本番へ適用しない)）。

## アプリに `/metrics` が無い

構成図は `Prometheus -.スクレイプ.-> api & prosody & DB` としているが、
**api と prosody に metrics の経路は実装されていない。**

| 案 | 取れるもの | アプリの改修 |
| --- | --- | --- |
| **A. blackbox / node exporter** | **外形の稼働率**、ホストの資源 | **不要** |
| B. `/metrics` を実装する | 要求数、応答時間、判定の内訳 | api と prosody に必要 |

**A を採る。** 運用で最初に要るのは「落ちていないか」であり、A で足りる。
B はアプリの改修を伴うため、別 Issue とする。

## やること

- [x] `k8s/monitoring/base/` にマニフェストを作る
- [x] `overlays/local`（k3d）と `overlays/prod`（VPS）を作る
- [x] k3d で動かし、実際に値が取れることを確かめる
- [x] README に手順を書く

## 完了条件

- [x] Prometheus が起動し、対象を検出している
- [x] **node-exporter が DaemonSet**（[下記](#node-exporter-を-daemonset-にする理由)）
- [x] **blackbox-exporter が公開中の2サイトを外形監視している**
- [x] Grafana が起動し、Prometheus をデータ源として認識している
- [x] Grafana に Ingress で到達できる
- [x] **Grafana の管理者パスワードが Secret**（リポジトリに入れない）
- [x] すべてのコンテナに `resources` がある
- [x] `./scripts/check.sh` が通る

## やらないこと

- **本番（VPS）への適用**（[下記](#本番へ適用しない)）
- `/metrics` の実装（別 Issue）
- アラートの通知先設定（Alertmanager）
- Grafana のダッシュボードの作り込み（データ源の接続まで）

## 実装上の注意

### 本番へ適用しない

本番は 575 が compose で動いており、そこへ k3s を足すと**二重になる**。
ADR-0007 の予算は 575 が k3s の中で動く前提だった。

現在の空きは 1.2 GiB で、k3s（約 500 MiB）と監視（約 330 MiB）を足すと
**判定基準の 400 MiB ちょうど**になる。適用は実測を経てから判断する。

### node-exporter を DaemonSet にする理由

ホストの CPU・メモリ・ディスクを取るため、**ノードごとに1つ必要**である。
Deployment だと同じノードに複数載ったり、載らないノードが出たりする。

単一ノードでは差が出ないが、ノードを増やしたときに自動で追随する。

### Grafana のパスワードは Secret にする

`kustomize` の `secretGenerator` で作る。ファイルは `.gitignore` で除外する
（アプリ側と同じ扱い）。

### Prometheus には RBAC が要る

Kubernetes の API を使って対象を検出するため、ServiceAccount と
ClusterRole が要る。**参照のみに絞る。**

## 参考

- [ADR-0007 の構成図](../adr/0007-hosting-conoha-vps.md#6-決定)
- [k8s/README.md](../../k8s/README.md)
