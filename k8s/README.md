# k8s

[ADR-0007](../docs/adr/0007-hosting-conoha-vps.md) が決めた **単一ノードの k3s** 向けのマニフェストです。

```
k8s/
├── base/                 共通の定義（環境に依存しない）
└── overlays/
    ├── local/            k3d 用。自分でビルドしたイメージを使う
    └── prod/             ConoHa VPS 用。GHCR のイメージを使う
```

`kustomize` を使います（`kubectl` に同梱。追加の導入は不要）。

## 秘密情報

**`secret.env` はコミットしません。** 手元で作ります。

```bash
cat > k8s/overlays/local/secret.env <<'ENV'
POSTGRES_PASSWORD=local-only-not-secret
API_DATABASE_URL=postgres://sns575:local-only-not-secret@postgres:5432/sns575?sslmode=disable
ENV
```

**2つの値は整合させてください。** `API_DATABASE_URL` の中のパスワードが
`POSTGRES_PASSWORD` と違うと、api だけが接続できません。

本番用は `k8s/overlays/prod/secret.env` に、生成した強いパスワードで作ります。

```bash
openssl rand -base64 32
```

## ローカルで動かす（k3d）

**本番と同じ k3s を Docker の中で動かします。** kind（upstream の Kubernetes）ではなく
k3d を使うのは、内蔵の Traefik や local-path-provisioner まで本番と揃えるためです。

```bash
brew install k3d
```

```bash
k3d cluster create sns575 --agents 0 --port "8088:80@loadbalancer"
```

**イメージは自分でビルドしたものを取り込みます。** GHCR のイメージは linux/amd64 で、
開発機が Apple Silicon の場合はエミュレーションになります。

```bash
docker compose -f compose.yaml build
k3d image import 575-web:runtime 575-api:runtime 575-prosody:runtime -c sns575
```

```bash
kubectl apply -k k8s/overlays/local
kubectl -n sns575 get pods -w
```

すべてが Ready になったら開きます。

```bash
curl -sI http://575.localhost:8088/
```

E2E テストもこのクラスタに対して実行できます。

```bash
E2E_BASE_URL=http://575.localhost:8088 npm test --prefix e2e
```

後片付けは次のとおりです。

```bash
k3d cluster delete sns575
```

## 本番へ適用する（第2期）

**まだ実施していません。** 本番は現在 compose で動いています（[README](../README.md#本番環境)）。

```bash
kubectl apply -k k8s/overlays/prod
```

## 設計上の判断

### `replicas` を base に書かない

`base` に決め打ちの値があると、環境ごとに patch で上書きすることになり、
どちらが効いているか読み取りにくくなります。`overlays` の `replicas` で与えます。

| 環境 | web / api / prosody |
| --- | --- |
| local | 1 |
| prod | 2 |

### `namespace` を base に書かない

`base` に `namespace` を書くと、そこにある資源だけが先に名前空間を持ちます。
`overlays` の `secretGenerator` が作る Secret とは名前空間が食い違い、
**`secretKeyRef` の名前がハッシュ付きに書き換わらず、存在しない Secret を参照します。**

`namespace` は `overlays` で与えます。

### `enableServiceLinks: false`

Kubernetes は既定で、Service 名から環境変数を注入します。
Service `prosody` に対して `PROSODY_PORT=tcp://10.43.x.x:8000` が入り、
**アプリ自身の `PROSODY_PORT`（ポート番号）を上書きして起動に失敗しました。**

すべての Pod で無効にしています。

### migrate は postgres を待つ

`kubectl apply` は順序を保証しません。待たないと `connection refused` で失敗します。
`initContainer` で `pg_isready` を待ちます。

### `preStop` で Endpoints から外れるのを待つ

Pod の削除は「Endpoints の更新」と「SIGTERM」が並行して進みます。
待たないと Ingress が終了中の Pod に転送し、**502 になります**（実際に発生しました）。

web と prosody には `preStop` に `sleep 5` を置いています。
**api には置けません。** runtime が distroless でシェルも `sleep` も無いためです。
代わりに SIGTERM を受けて Echo を graceful shutdown しています（`cmd/api/main.go`）。
