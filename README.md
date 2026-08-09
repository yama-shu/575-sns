# 575 — 五七五でしか投稿できないSNS

> 言いたいことは、五七五で。

**575** は、投稿本文が五七五（音数律）に収まっていないと投稿できない SNS です。
俳句SNSではありません。季語も切れ字も不要で、投稿する内容は何でも構いません。
制約は「形式」だけです。多少の字余り・字足らずは許容します。

```
     入力: 「今日もまた 会議のための 会議かな」
     判定: ✅ 定型（5 / 7 / 5）→ 投稿可能

     入力: 「今日は疲れた」
     判定: ❌ 破調（7モーラ）→ 投稿不可
```

---

## 現在のステータス

| フェーズ | 状態 |
| --- | --- |
| 要件定義 | ✅ 完了 |
| 基本設計 | ✅ 完了 |
| 詳細設計 | ✅ 完了 |
| 実装 | 🔵 進行中（M0 開発基盤） |

現在は **M0（開発基盤）** の段階です。各サービスは起動と疎通確認ができる骨組みまでで、
五七五の判定ロジックは未実装です（[M1 のチケット](https://github.com/yama-shu/575-sns/milestone/2)で実装します）。

---

## ドキュメント

ドキュメントの全体地図は **[docs/README.md](docs/README.md)** にあります。
主要な入口は以下の3つです。

| 入口 | 内容 |
| --- | --- |
| [要件定義書](docs/requirements/01-requirements.md) | 何を作るのか。背景・ペルソナ・ユースケース・機能要件・非機能要件 |
| [Architecture Decision Records](docs/adr/README.md) | なぜそう決めたのか。複数案の比較とトレードオフの記録 |
| [Issue 起票ドラフト](docs/issues/README.md) | GitHub Issues に起票する前の下書き置き場 |

---

## リポジトリ構成

```
.
├── README.md                 # このファイル
├── LICENSE
├── compose.yaml              # ローカル実行用（本番相当のイメージで動かす）
├── compose.override.yaml     # 開発用の差分（ホットリロード等。up で自動適用）
├── .github/workflows/        # CI
├── scripts/                  # 省力化スクリプト（check.sh など）
├── .env.example              # 環境変数の雛形。cp して .env を作る
├── web/                      # TypeScript / Next.js — 画面描画・SSR
├── api/                      # Go / Echo — 業務ロジック・永続化
├── prosody/                  # Python / FastAPI — 五七五判定エンジン
└── docs/
    ├── README.md             # ドキュメント地図（ここから読む）
    ├── requirements/         # 要件定義
    ├── design/
    │   ├── basic/            # 基本設計（構成図・ER図・画面遷移・API一覧）
    │   └── detail/           # 詳細設計（アルゴリズム・クラス設計・エラー設計・テスト設計）
    ├── adr/                  # Architecture Decision Record（意思決定の記録）
    └── issues/               # GitHub Issue 起票用ドラフト
```

サービスを3つに分けている理由は [ADR-0002](docs/adr/0002-tech-stack.md) にあります。
要点は、形態素解析の辞書が数十〜数百 MB をメモリに常駐させるため、
判定エンジンを api に同居させると api を増やすたびに辞書が複製されることです。

---

## 開発環境

### 必要なもの

**Docker と Docker Compose だけあれば動きます。** 各言語の処理系をホストへ入れる必要はありません。

| ツール | バージョン | 用途 |
| --- | --- | --- |
| Docker Engine | 24.0 以上 | 全サービスの実行 |
| Docker Compose | v2.21 以上 | 複数サービスの起動 |

> **メモリ**: Docker に **4 GB 以上**を割り当ててください。
> prosody が形態素解析の辞書をメモリへ展開するため、既定の 2 GB では起動に失敗することがあります。

各サービスが使う処理系は以下です。**コンテナ内に閉じている**ため、
ホストへのインストールは不要です（エディタの補完を効かせたい場合のみ入れてください）。

| サービス | 言語 | 主要ライブラリ |
| --- | --- | --- |
| web | TypeScript / Node.js 24 (LTS) | Next.js 16 / React 19 |
| api | Go 1.26 | Echo v4 / pgx v5 |
| prosody | Python 3.13 | FastAPI / SudachiPy + SudachiDict-core |
| db | PostgreSQL 18 | — |

### 構築手順

```bash
git clone git@github.com:yama-shu/575-sns.git
cd 575-sns
cp .env.example .env        # 既定値のままで動くため、編集は任意
docker compose up --build
```

初回はイメージのビルドと辞書の取得で 5〜10 分ほどかかります。2回目以降はキャッシュが効きます。

起動したら http://localhost:3000 を開いてください。
各サービスの疎通状況が表示されます。

環境が壊れたときは、次のコマンドで完全に破棄してから作り直せます。

```bash
docker compose down -v      # コンテナとボリューム（DB のデータ）を削除する
docker compose up --build
```

### 実行方法

| コマンド | 動作 |
| --- | --- |
| `docker compose up` | **開発用**。ソースを編集すると自動で反映される（ホットリロード） |
| `docker compose -f compose.yaml up --build` | **本番相当**。本番と同じイメージで動かす |
| `docker compose logs -f prosody` | 特定サービスのログを追う |
| `docker compose down` | 停止する（DB のデータは残る） |
| `docker compose down -v` | 停止してデータも破棄する |

`docker compose up` は [compose.override.yaml](compose.override.yaml) を自動的に読み込みます。
開発時にだけ必要なもの（ホットリロード用のバインドマウント、デバッグ用のポート公開）は
すべてそちらに寄せてあるため、**本番イメージに開発ツールが混入しません**。

2つのモードは**別のイメージタグ**（`575-web:dev` と `575-web:runtime`）を使います。
同じタグを共有すると、最後にビルドした方のイメージが両モードで使われてしまい、
開発モードで起動したはずが本番イメージ（`node server.js`）が動く、という事故が起きます。

> モードを切り替えるときは、先に `docker compose down` してください。
> 同じホストポートを使うため、両方を同時には起動できません。

### 公開されるポート

| URL | サービス | 備考 |
| --- | --- | --- |
| http://localhost:3000 | web | |
| http://localhost:8080/readyz | api | 依存先（db / prosody）への疎通状況を返す |
| http://localhost:8000/readyz | prosody | **開発時のみ**公開。本番はクラスタ内部からのみ到達可能 |
| http://localhost:8000/docs | prosody | 判定 API のドキュメント（開発時のみ） |
| `localhost:5432` | db | **開発時のみ**公開 |

疎通はコマンドラインからも確認できます。

```bash
curl -s localhost:8080/readyz | jq
# {"dependencies":{"database":true,"prosody":true},"ready":true}
```

### 判定を試す

```bash
curl -s -X POST localhost:8000/v1/analyze \
  -H 'Content-Type: application/json' \
  -d '{"text":"今日もまた会議のための会議かな"}' | jq
```

```json
{
  "verdict": "teikei",
  "normalized_text": "今日もまた会議のための会議かな",
  "total_mora": 17,
  "segments": [
    { "text": "今日もまた",   "mora": 5, "expected": 5, "diff": 0 },
    { "text": "会議のための", "mora": 7, "expected": 7, "diff": 0 },
    { "text": "会議かな",     "mora": 5, "expected": 5, "diff": 0 }
  ]
}
```

**破調でも 200 が返ります。** 判定を求められて判定を返しているためです。
「破調だから投稿を拒否する」のは 575 の業務ルールで、api の責務です。

```bash
curl -s -X POST localhost:8000/v1/analyze \
  -H 'Content-Type: application/json' \
  -d '{"text":"今日は疲れた"}' | jq -c '{verdict, reason, total_mora}'
# {"verdict":"hacho","reason":"TOO_FEW_MORA","total_mora":7}
```

### API 定義

判定 API の契約は [prosody/openapi.json](prosody/openapi.json) にあります。
api（Go）と web（TypeScript）はこれを契約として型を生成するため、
**定義を変えたらコミットしてください**。CI が最新かどうかを検査します。

```bash
./scripts/openapi.sh          # 書き出す
./scripts/openapi.sh --check  # 最新か確かめる
```

### 設定方法

設定値はコードに埋め込まず、**すべて環境変数**から読み込みます。
ローカルでは [.env](.env.example) で、本番では Kubernetes の Secret / ConfigMap で与えます。

`.env` が無くても [compose.yaml](compose.yaml) の既定値で起動します。変えたい項目だけ書いてください。

| 環境変数 | 既定値 | 説明 |
| --- | --- | --- |
| `POSTGRES_USER` | `sns575` | DB のユーザ名 |
| `POSTGRES_PASSWORD` | `local-dev-only` | DB のパスワード。**ローカル専用の値** |
| `POSTGRES_DB` | `sns575` | DB 名 |
| `POSTGRES_PORT` | `5432` | ホスト側に公開するポート |
| `API_PORT` | `8080` | ホスト側に公開するポート |
| `API_DATABASE_URL` | compose が組み立て | DB 接続文字列。**パスワードを含むためログに出さない** |
| `API_PROSODY_URL` | `http://prosody:8000` | prosody の場所 |
| `API_DATABASE_TIMEOUT` | `3s` | DB のタイムアウト |
| `API_PROSODY_TIMEOUT` | `1s` | prosody のタイムアウト（[基本設計 01 §6](docs/design/basic/01-architecture.md#6-サービス間通信)） |
| `API_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `PROSODY_PORT` | `8000` | ホスト側に公開するポート |
| `PROSODY_LOG_LEVEL` | `info` | ログレベル |
| `PROSODY_SUDACHI_DICT` | `core` | 使用する SudachiDict（[ADR-0003](docs/adr/0003-morphological-analyzer.md)） |
| `PROSODY_WORKERS` | `1` | 本番のワーカープロセス数。判定は CPU バウンドのため CPU 数に合わせる |
| `WEB_PORT` | `3000` | ホスト側に公開するポート |
| `WEB_API_INTERNAL_URL` | `http://api:8080` | SSR 時に web が api を呼ぶ先 |

> **認証情報をコミットしないでください**（[NFR-04-07](docs/requirements/01-requirements.md#nfr-04-セキュリティ)）。
> `.env` は [.gitignore](.gitignore) で除外済みです。`.env.example` には実在する値を書かないでください。

### ビルド方法

```bash
docker compose -f compose.yaml build          # 本番相当のイメージを全サービス分
docker compose -f compose.yaml build prosody  # 特定のサービスだけ
```

各 Dockerfile はマルチステージ構成です。`dev` と `runtime` を分けているため、
本番イメージにはコンパイラ・テストツール・ホットリロードの仕組みが入りません。

### テスト方法

判定ロジックは M1 で実装します。現時点では起動確認のテストのみです。
テストケースの設計は [詳細設計 04](docs/design/detail/04-test-design.md) にあります。

```bash
# prosody — pytest（カバレッジは pyproject.toml の設定で常時計測される）
docker compose run --rm prosody pytest

# api — go test
docker compose run --rm api go test ./... -cover

# web — 型検査と lint
docker compose run --rm web npm run typecheck
docker compose run --rm web npm run lint
```

カバレッジの目標は [NFR-05-04](docs/requirements/01-requirements.md#nfr-05-保守性運用性) にもとづき、
prosody が C1 100%、api の usecase 層が 90% です。

#### CI と同じ検査をまとめて実行する

lint・型検査・テストを CI と同じ内容で流せます。**push する前にこれを通してください。**

```bash
./scripts/check.sh              # 3サービスすべて
./scripts/check.sh prosody      # サービスを指定
./scripts/check.sh api web
```

使うイメージとツールのバージョンは [CI](.github/workflows/ci.yml) と揃えてあるため、
「手元では通ったのに CI で落ちる」が起きません。

### CI

Pull Request を作ると [CI](.github/workflows/ci.yml) が自動で走ります。

| ジョブ | 内容 |
| --- | --- |
| 変更範囲の判定 | 変更されたサービスだけを実行する。共有の定義が変わった場合は全サービス |
| prosody | ruff / ruff format / mypy / pytest（**C1 100% を下回ると失敗**）/ pip-audit |
| api | gofmt / go vet / golangci-lint / go test / govulncheck |
| web | eslint / tsc / next build / npm audit |
| イメージのビルド | 各サービスの `runtime` を **x86_64 ランナー**でビルドする |
| カバレッジの報告 | 結果を PR に1つのコメントとして投稿・更新する |
| CI | 上記を集約する。ブランチ保護の required status check はこれを指定する |

**イメージのビルドを x86_64 で行う理由**は、本番が ConoHa VPS の x86_64 で動くためです
（[ADR-0007](docs/adr/0007-hosting-conoha-vps.md)）。アーキテクチャが違うランナーで検証すると、
本番でのみ壊れる依存を見逃します。

### リリース方法

本番へのデプロイは M5 で整備します（[未起票のバックログ](docs/issues/README.md#未起票のバックログ)）。
現時点でリリース版のイメージを手元で作る手順は以下です。

```bash
# 本番相当のイメージをタグ付きでビルドする
docker build -t 575-sns/prosody:$(git describe --tags --always) --target runtime ./prosody
docker build -t 575-sns/api:$(git describe --tags --always)     --target runtime ./api
docker build -t 575-sns/web:$(git describe --tags --always)     --target runtime ./web
```

本番は x86_64（ConoHa VPS）で動きます（[ADR-0007](docs/adr/0007-hosting-conoha-vps.md)）。
開発機が Apple Silicon の場合はローカルで ARM のイメージができるため、
本番と同じものを確認したい場合は `--platform linux/amd64` を付けてください。

---

## ライセンス

[MIT License](LICENSE)
