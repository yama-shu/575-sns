| 項目 | 値 |
| --- | --- |
| タイトル | `infra: イメージを GHCR へ配布し、サーバーは pull だけにする` |
| ラベル | `infrastructure` |
| マイルストーン | M5 公開 |
| 起票済み | [#86](https://github.com/yama-shu/575-sns/issues/86) |

---

## 背景・目的

[ADR-0007](../adr/0007-hosting-conoha-vps.md) は配布の方法を決めている。

> **イメージビルド** | GitHub Actions → GHCR、サーバーは pull のみ |
> Next.js のビルドは 1 GB 級を要求する。サーバー上でビルドすると確実に OOM する

**決めたが、まだ作っていない。** [#84](https://github.com/yama-shu/575-sns/issues/84) の公開手順は
`docker compose up -d --build` になっており、サーバー上でビルドする形だった。

公開（2026-08-20 目標）の前に、配布の経路を用意する。

## なぜサーバー上でビルドしないのか

| 観点 | サーバー上でビルド | GHCR から pull |
| --- | --- | --- |
| 所要 | 15〜25分 | 数分 |
| CPU | **2 vCPU を使い切る**。同居する別プロジェクトを巻き込む | ほぼ使わない |
| メモリ | Next.js のビルドが 1 GB 級を要求する | 不要 |
| 更新のたび | 毎回ビルドし直す | pull するだけ |

**CI は既に x86_64 で `runtime` ステージをビルドしている**（`images` ジョブ）。
検証のために捨てているものを、そのまま配布に使う。

## やること

- [ ] `images` ジョブから GHCR へ push する
- [ ] 本番用の compose ファイルを足す
- [ ] README に pull による起動手順を書く
- [ ] [#84](https://github.com/yama-shu/575-sns/issues/84) の手順を pull に差し替える

## 完了条件

- [ ] main への push で GHCR にイメージが上がる
- [ ] **Pull Request では push しない**（[下記](#pull-request-では-push-しない)）
- [ ] `latest` とコミットの SHA の2つのタグが付く
- [ ] タグを指定して**過去の版に戻せる**（[下記](#戻せることを配布の要件にする)）
- [ ] サーバーが `pull` だけで起動できる
- [ ] `compose.yaml` を変更していない（[下記](#composeyaml-を変えない)）
- [ ] `./scripts/check.sh` が通る

## やらないこと

- **自動デプロイ**（サーバーへの push 型 CD。[下記](#自動デプロイまでは作らない)）
- イメージの脆弱性スキャン（別途）
- マルチアーキテクチャ対応（本番は x86_64 のみ）
- k3s 用のマニフェスト（公開後）

## 実装上の注意

### `compose.yaml` を変えない

`compose.yaml` は**ローカルで本番相当を動かすため**のファイルであり、
E2E もこれを使っている。GHCR の名前を書き込むと、手元でビルドできなくなる。

`compose.prod.yaml` を足し、`image` だけを差し替える。

```bash
docker compose -f compose.yaml -f compose.prod.yaml pull
docker compose -f compose.yaml -f compose.prod.yaml up -d --no-build
```

### Pull Request では push しない

`images` ジョブは PR でも動く。**PR の内容が `latest` になると、
まだレビューされていないイメージが配布される。**

push するのは `main` への push のときだけにする。PR では従来どおりビルドの検証だけを行う。

### 戻せることを配布の要件にする

`latest` だけだと、壊れた版を配布したときに戻せない。
コミットの SHA を第2のタグとして付け、`IMAGE_TAG` で選べるようにする。

```bash
IMAGE_TAG=<SHA> docker compose -f compose.yaml -f compose.prod.yaml up -d --no-build
```

### 自動デプロイまでは作らない

サーバーへ自動で反映する仕組みは、公開後に検討する。

**先に配布の経路だけを作る。** 公開当日は手で `pull` する。
自動化は、手順が固まってからでないと、壊れたときに何が起きたか分からなくなる。

## 参考

- [ADR-0007: ホスティング先](../adr/0007-hosting-conoha-vps.md)
- [#84: 575 を HTTPS で一般公開する](https://github.com/yama-shu/575-sns/issues/84)
