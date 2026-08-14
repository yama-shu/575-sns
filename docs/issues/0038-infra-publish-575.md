| 項目 | 値 |
| --- | --- |
| タイトル | `infra: 575 を HTTPS で一般公開する` |
| ラベル | `infrastructure` |
| マイルストーン | M5 公開 |
| 起票済み | [#84](https://github.com/yama-shu/575-sns/issues/84) |

---

## 背景・目的

575 を**第三者が利用できる形で公開する**。

[ADR-0007](../adr/0007-hosting-conoha-vps.md) の移行先である ConoHa VPS 2 GB を **2026-08-16 に新規契約する**予定である。
そこに oil_game を移設したあと、575 を公開する。

**公開日を 2026-08-20 に置く。** k3s と監視の構築を公開の前提にしない。
先に公開し、目標構成へは公開後に到達する（[下記](#k3s-を公開の前提にしない)）。

## 決めること（着手前）

| 項目 | 選択肢 | 状態 |
| --- | --- | --- |
| サブドメイン | **`575.ramen-oil.com`** | 決定（[下記](#サブドメインは-575ramen-oilcom)） |
| アクセス数の記録 | Cloudflare を前段に置く / nginx の `access_log` を集計する | **保留**（[下記](#アクセス数の記録は保留する)） |

## やること

- [ ] DNS に A レコードを追加する
- [ ] `.env` を本番の値で用意する（[下記](#env-で必ず変える3項目)）
- [ ] `docker compose -f compose.yaml` で起動する
- [ ] nginx に vhost を足す（[下記](#nginx-は既存のものに相乗りする)）
- [ ] certbot でサブドメインの証明書を取得する
- [ ] 運営アカウントを付与する（[下記](#運営アカウントは-db-を直接更新する)）
- [ ] 初期の句を投稿する
- [ ] 外形監視を開始する
- [ ] README に本番の情報を追記する

## 完了条件

### 公開

- [ ] `https://575.ramen-oil.com` が **200** を返す
- [ ] HTTP でアクセスすると HTTPS へリダイレクトされる
- [ ] 証明書の発行者が Let's Encrypt である
- [ ] **登録 → 投稿 → 閲覧が実際に通る**（本番環境で手で確認する）
- [ ] 判定（prosody）が本番でも動く

### 安全

- [ ] **api と web のポートが外部に開いていない**（[下記](#api-のポートが公開されている)）
- [ ] `API_SECURE_COOKIE=true` になっている
- [ ] DB のパスワードが既定値のままでない
- [ ] nginx に `limit_req` を入れている（[下記](#レート制限が無い)）

### oil_game を壊さない

- [ ] `nginx -t` が通る
- [ ] **ramen-oil.com が変わらず 200 を返す**
- [ ] メモリに余裕がある（`free -h`）

### 記録

- [ ] 作業のログを本 Issue に残す
- [ ] 公開後の記録を取得する（証明書・稼働率）

## やらないこと

- k3s の構築（公開後。[下記](#k3s-を公開の前提にしない)）
- 監視（Prometheus + Grafana）の構築（公開後）
- CD パイプライン（初回は手動で構築する）
- バックアップの自動化（公開後。ただし手動の `pg_dump` は取る）
- レート制限の実装（[NFR-04-05](../requirements/01-requirements.md#nfr-04-セキュリティ)。nginx 側で緩和するに留める）
- アクセス数の記録（[下記](#アクセス数の記録は保留する)）

## 実装上の注意

### k3s を公開の前提にしない

提出は **2026-09-01** であり、契約から16日しかない。

| 進め方 | 公開日 | 提出時点の運用期間 |
| --- | --- | --- |
| **移行 → compose で公開 → 後から k3s** | 8/20 | **12日** |
| 移行 → k3s → 監視 → 公開 | 8/28〜31 | 1〜4日 |

**運用の実績は公開日からしか積み上がらない。** 証明書の更新も稼働率も、
公開していない期間には作れない。

**ADR-0007 の決定を変えるものではない。** 目標構成は 2 GB + k3s のままで、
到達の順序を変える。

### メモリは compose のみなら足りる

[ADR-0007 §7](../adr/0007-hosting-conoha-vps.md#7-メモリ予算) の予算から
k3s（約 500 MiB）と Prometheus + Grafana（約 330 MiB）を外すと、約 830 MiB が空く。

```
OS 250 + Docker/oil_game 93 + web 35 + api 3 + prosody 128 + postgres 40 ≈ 550 MiB
残り約 1,400 MiB（+ swap 2 GB）
```

**イメージのビルドはこの余裕を使う。** 手元の環境で web の `runtime` ステージのビルドに
9分かかった実測がある。swap を確保したうえで実施する。

### `.env` で必ず変える3項目

`compose.yaml` の既定値は**ローカル開発専用**である。

| 変数 | 既定 | 本番 |
| --- | --- | --- |
| `POSTGRES_PASSWORD` | `local-dev-only` | **変える** |
| `API_SECURE_COOKIE` | `false` | **`true`**（[README](../../README.md) の設定表） |
| `WEB_PORT` / `API_PORT` | `3000` / `8080` | **`127.0.0.1:3000` / `127.0.0.1:8080`**（下記） |

### api のポートが公開されている

`compose.yaml` は api を `"${API_PORT:-8080}:8080"` で公開している。
これは **0.0.0.0 に束縛される**ため、そのまま起動すると
**api が HTTPS を経由せずインターネットから直接叩ける**。

ブラウザは web としか通信しない（[基本設計 01 §6](../design/basic/01-architecture.md#6-サービス間通信)）ため、
api を外に出す必要はない。

`.env` で束縛先を含めて指定する。`compose.yaml` の変更は要らない。

```
WEB_PORT=127.0.0.1:3000
API_PORT=127.0.0.1:8080
```

`"127.0.0.1:3000:3000"` に展開され、nginx からは届き、外からは届かなくなる。

### サブドメインは `575.ramen-oil.com`

取得済みの `ramen-oil.com` のサブドメインを使う。**新しくドメインは取らない。**

- DNS は お名前.com（ネームサーバーは `dnsv.jp`）で管理している
- `575.ramen-oil.com` は未使用である
- 数字で始まるラベルは RFC 1123 が許しており、TLD が `com` のため IP と混同されない

**公開後に変えない。** URL を変えると証明書の履歴（crt.sh）も稼働率の記録も分散し、
運用の実績が2つに割れる。

### nginx は既存のものに相乗りする

既存の VPS では **nginx が 443 を使用している**（oil_game のリバースプロキシ）。
Caddy を足すと競合する（[#82](https://github.com/yama-shu/575-sns/issues/82)）。

vhost を1つ足し、`localhost:3000` へ流す。

### レート制限が無い

[NFR-04-05](../requirements/01-requirements.md#nfr-04-セキュリティ) は未実装である。
oil_game の `error.log` には `/.env` `/wp-admin/install.php` `/config.php.bak` への
無差別スキャンが実際に記録されていた。**公開すれば 575 にも来る。**

実装は別 Issue とし、**公開時は nginx の `limit_req` で緩和する**。
とくに登録・ログイン（bcrypt コスト 12）は CPU を使うため、ここが無防備だと
判定エンジンごと巻き込まれる。

### prosody の起動を待つ

辞書のロードに時間がかかる。`compose.yaml` の healthcheck は
`start_period: 60s` を置いている。**起動直後の 503 は異常ではない。**

api は prosody が healthy になるまで起動しない（`depends_on`）。

### 運営アカウントは DB を直接更新する

`is_admin` を与える API は無い（[#74](https://github.com/yama-shu/575-sns/issues/74) の判断）。
**付与しないと、通報が来ても裁けない。**

```sql
UPDATE users SET is_admin = true WHERE handle = '...';
```

### 初期の句を入れる

空のタイムラインで公開すると、開いた人に何も見えない。

### 記録を残す

稼働率は**公開初日から取らないと後から作れない**。外形監視は公開と同じ日に始める。

### アクセス数の記録は保留する

方式を決めていないため、本 Issue では扱わない。

**遡って作れない種類の記録である。** nginx の `access_log` は公開初日から残るが、
`log_format` を変えるとそれ以前の行と揃わない。Cloudflare を前段に置く場合は、
置いた日からしか集計されない。**決めた日が記録の開始日になる。**

## 手順

### 1. 事前（8/16 まで）

- [ ] `ramen-oil.com` の A レコードの TTL を 3600 → 300 に下げる（切り替えを速くする）
- [ ] `.env` の内容を決める
- [ ] nginx の vhost 設定を書いておく
- [ ] 外形監視のアカウントを用意する

### 2. 公開当日

```bash
# 1) 取得
git clone https://github.com/yama-shu/575-sns.git
cd 575-sns

# 2) .env（本番の値）
cp .env.example .env
# POSTGRES_PASSWORD / API_SECURE_COOKIE / WEB_PORT / API_PORT を編集

# 3) 起動（compose.override.yaml を読ませない）
docker compose -f compose.yaml up -d --build --wait
docker compose -f compose.yaml ps

# 4) 内部から疎通
curl -sI http://127.0.0.1:3000/ | head -3
curl -s  http://127.0.0.1:8080/readyz

# 5) nginx
sudo nginx -t && sudo systemctl reload nginx

# 6) 証明書
sudo certbot --nginx -d 575.ramen-oil.com

# 7) 外から
curl -sI https://575.ramen-oil.com | head -3
```

### 3. 公開後

- [ ] 運営アカウントの付与
- [ ] 初期の句の投稿
- [ ] 外形監視の開始
- [ ] `pg_dump` を1回取る
- [ ] `restart: unless-stopped` が効いていることの確認（`docker inspect`）
- [ ] TTL を 3600 に戻す

## 参考

- [ADR-0007: ホスティング先](../adr/0007-hosting-conoha-vps.md)
- [基本設計 01 §6: サービス間通信](../design/basic/01-architecture.md#6-サービス間通信)
- [README: 設定方法](../../README.md)
