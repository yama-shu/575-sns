| 項目 | 値 |
| --- | --- |
| タイトル | `infra: K8s マニフェストを作成し、ローカルの k3d で実証する` |
| ラベル | `infrastructure` |
| マイルストーン | M5 公開 |
| 起票済み | [#91](https://github.com/yama-shu/575-sns/issues/91) |

---

## 背景・目的

[ADR-0007](../adr/0007-hosting-conoha-vps.md) は本番を **単一ノードの k3s** と決めている。
現在は compose で公開しており（[#84](https://github.com/yama-shu/575-sns/issues/84)）、**マニフェストが存在しない。**

**ローカルで作り、ローカルで実証する。** 本番の VPS を使わずに完結するため、
移設や公開の状態に依存せず進められる。

## 使う道具を決める

### k3d を使う（kind ではない）

| 候補 | 判断 |
| --- | --- |
| **k3d** | 採用。**k3s を Docker の中で動かす**。本番と同じ k3s であり、内蔵の Traefik・local-path-provisioner も同じ |
| kind | 採らない。upstream の Kubernetes であり、Ingress Controller を別途入れることになる。本番との差が出る |
| minikube | 採らない。仮想マシンを別に持つ構成が重く、k3s でもない |

**本番が k3s である以上、検証も k3s で行う。** 「ローカルでは通ったが本番で通らない」を
減らすことが、この作業の目的そのものである。

### kustomize を使う（Helm ではない）

| 候補 | 判断 |
| --- | --- |
| **kustomize** | 採用。`kubectl` に同梱され、追加の実行環境が要らない。`base` と `overlays` で差分だけを持てる |
| Helm | 採らない。テンプレート言語と値ファイルの学習・保守が要る。**単一プロダクトを1箇所に置くだけ**であり、テンプレート化の必要が薄い |
| 素のマニフェストを環境ごとに複製 | 採らない。同じ内容が2箇所に増え、片方だけ直す事故が起きる |

## やること

- [x] `k8s/base/` にマニフェストを作る
- [x] `k8s/overlays/local/`（k3d 用）と `k8s/overlays/prod/`（VPS 用）を作る
- [x] k3d で実際に動かし、導線が通ることを確かめる
- [x] `replicas` を変えて動くことを確かめる
- [x] README に手順を書く

## 完了条件

### 構成

- [x] postgres が **StatefulSet + PVC**（[ADR-0007 の構成図](../adr/0007-hosting-conoha-vps.md#6-決定)）
- [x] マイグレーションが **Job**（api の起動時に適用しない。[基本設計 03 §6](../design/basic/03-database.md)）
- [x] 期限切れセッションの削除が **CronJob**（[ADR-0006](../adr/0006-authentication.md)）
- [x] Ingress で web に到達する
- [x] **すべてのコンテナに `resources` がある**（[下記](#resources-を必ず付ける)）
- [x] **`replicas` が決め打ちでない**（[下記](#replicas-を決め打ちにしない)）
- [x] 秘密情報がリポジトリに入っていない（[下記](#secret-をコミットしない)）

### 実証（k3d 上）

- [x] すべての Pod が Ready になる
- [x] Ingress 経由で **登録 → 投稿 → 閲覧**が通る
- [x] 判定（prosody）が動く
- [x] `replicas` を 1 / 2 / 3 と変えても動く
- [x] Job を再実行しても壊れない（マイグレーションの冪等性）
- [x] Pod を削除すると再作成され、復帰する

### 共通

- [x] `./scripts/check.sh` が通る

## やらないこと

- **本番（VPS）への適用**（第2期。本 Issue はローカルでの実証まで）
- 監視（Prometheus + Grafana）のマニフェスト
- TLS / cert-manager（現在は nginx + certbot で終端している。k3s へ移す際に決める）
- HorizontalPodAutoscaler（単一ノードで CPU の余裕が無く、意味がない）
- CI での自動適用

## 実装上の注意

### ローカルでは自分でビルドしたイメージを使う

GHCR に置いてあるイメージは **linux/amd64** である（本番が x86_64 のため）。
開発機が Apple Silicon の場合、そのまま k3d で動かすとエミュレーションになる。

**`overlays/local` はローカルでビルドしたイメージを参照し、`k3d image import` で
クラスタに取り込む。** `overlays/prod` は GHCR を参照する。

### `resources` を必ず付ける

[ADR-0007](../adr/0007-hosting-conoha-vps.md#この決定によって諦めること抱えるリスク) が
「`resources` / `mem_limit` による相互保護」をリスクへの対処としている。
**1つの Pod がメモリを食い潰して他を巻き込むことを防ぐ。**

値は実測に基づく（[ADR-0007 §10](../adr/0007-hosting-conoha-vps.md#10-その後の経過)）。

| コンテナ | 実測 |
| --- | ---: |
| prosody | 126.2 MiB |
| web | 76.1 MiB |
| postgres | 46.8 MiB |
| api | 3.4 MiB |

### `replicas` を決め打ちにしない

ADR-0007 の検証項目に「`replicas` を増減しても正しく動作するか」がある。

`base` では `replicas` を指定せず、`overlays` の patch で与える。
**セッションや判定結果を Pod のメモリに持たないこと**を確かめる
（[NFR-03-02](../requirements/01-requirements.md#nfr-03-拡張性)）。

### Secret をコミットしない

DB のパスワードをリポジトリに入れない
（[NFR-04-07](../requirements/01-requirements.md#nfr-04-セキュリティ)）。

`kustomize` の `secretGenerator` で、`.env` 形式のファイルから生成する。
そのファイルは `.gitignore` に入れる。

### prosody は `PROSODY_WORKERS=1` にする

[ADR-0007](../adr/0007-hosting-conoha-vps.md#prosody_workers1--replicas2-とする理由) のとおり、
ワーカーごとに辞書が複製されるため、並列度はレプリカで取る。

## 参考

- [ADR-0007: ホスティング先](../adr/0007-hosting-conoha-vps.md)
- [基本設計 01: アーキテクチャ](../design/basic/01-architecture.md)
- [ADR-0006: 認証方式](../adr/0006-authentication.md)（セッション削除の CronJob）
