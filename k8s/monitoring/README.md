# k8s/monitoring

[ADR-0007 の構成図](../../docs/adr/0007-hosting-conoha-vps.md#6-決定)に置いた監視のマニフェストです。

```
k8s/monitoring/
├── base/
│   ├── prometheus*.yaml     収集と保存（RBAC・設定・本体）
│   ├── blackbox.yaml        外形監視
│   ├── node-exporter.yaml   ホストの資源（DaemonSet）
│   ├── grafana.yaml         可視化
│   └── ingress.yaml         Grafana のみ外に出す
└── overlays/{local,prod}/
```

**本番へはまだ適用していません。** ローカルの k3d で実証するところまでです。

## 何を監視するか

**アプリに `/metrics` はありません。** そのため、アプリの改修なしで取れるものから始めています。

| 対象 | 取れるもの |
| --- | --- |
| blackbox-exporter | **公開中のサイトの死活**、応答時間、**HTTPS 証明書の残り日数** |
| node-exporter | ホストの CPU・メモリ・ディスク |

要求数や判定の内訳は `/metrics` の実装が要ります（別 Issue）。

## ローカルで動かす

アプリ側のクラスタ（[k8s/README.md](../README.md)）と同じ k3d を使います。

```bash
cat > k8s/monitoring/overlays/local/secret.env <<'ENV'
GF_SECURITY_ADMIN_PASSWORD=local-only-not-secret
ENV
```

```bash
kubectl apply -k k8s/monitoring/overlays/local
kubectl -n monitoring get pods -w
```

Grafana は Ingress から開けます（`admin` と上のパスワード）。

```bash
open http://grafana.localhost:8088
```

Prometheus は外に出していないため、見るときは転送します。

```bash
kubectl -n monitoring port-forward svc/prometheus 9090:9090
```

## 設計上の判断

### Prometheus を外に出さない

Prometheus には認証がありません。Ingress に載せると**監視の内容を誰でも見られます。**
Grafana だけを出し、Prometheus は `port-forward` で見ます。

### node-exporter を DaemonSet にする

ホストの値を取るため、**ノードごとに1つ**必要です。Deployment では同じノードに
複数載ったり、載らないノードが出たりします。単一ノードでは差が出ませんが、
ノードを増やしたときに自動で追随します。

### 外形監視は blackbox-exporter から当てる

Prometheus が直接 URL を叩くのではなく、**blackbox-exporter に URL を渡して
結果を取り込みます**（`relabel_configs` で `__param_target` に載せ替え）。

証明書の残り日数（`probe_ssl_earliest_cert_expiry`）も同時に取れます。
**更新に失敗したまま気づかない事態を防げます。**

### 保持期間を 15 日にする

単一ノードで容量が限られます。長期の傾向より、直近の障害の追跡を優先します。

### Grafana のパスワードは Secret

`secretGenerator` で作り、ファイルは `.gitignore` で除外しています。
データ源の登録は起動時の provisioning で行い、画面から手で設定しません。
