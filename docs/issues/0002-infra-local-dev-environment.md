| 項目 | 値 |
| --- | --- |
| タイトル | `infra: docker compose でローカル開発環境を構築する` |
| ラベル | `infrastructure` |
| マイルストーン | M0 開発基盤 |
| 起票済み | [#2](https://github.com/yama-shu/575-sns/issues/2) |

---

## 背景・目的

575 は web / api / prosody / db の4コンポーネントで構成される。
これらが同時に立ち上がらないと動作確認ができない。

[ADR-0002](../adr/0002-tech-stack.md) では3サービス構成のデメリットとして
「ローカル環境の起動が面倒」を挙げ、その対処として
**`docker compose up` 一発で全サービスが立ち上がる状態を維持する**と決めている。
最初にこれを作らないと、以降のすべての開発が遅くなる。

## やること

- [ ] 各サービスの Dockerfile を作成する（web / api / prosody）
- [ ] ローカル実行用の `compose.yaml` を作成する
- [ ] 開発環境用（サンドボックス）の設定を、ローカル実行用とは別ファイルで用意する
- [ ] `.dockerignore` を作成する
- [ ] `.env.example` を作成する（実際の `.env` はコミットしない）
- [ ] README に開発環境の構築手順・実行方法・設定方法を追記する

## 完了条件

- [ ] リポジトリをクローンして `docker compose up` を実行するだけで、全サービスが起動する
- [ ] `http://localhost:3000` で web が表示される
- [ ] api から prosody・db へ疎通できる
- [ ] ソースコードを変更すると、コンテナを再ビルドせずに反映される（ホットリロード）
- [ ] `docker compose down -v` で完全に破棄し、再度 `up` で復旧できる
- [ ] README を読んだだけで、手順を知らない人が環境を構築できる
- [ ] 認証情報が一切コミットされていない

## やらないこと

- 本番環境向けのマニフェスト（別 Issue）
- CI の構築（別 Issue）

## 検討が必要な点

### ローカル実行用と開発環境用を分ける

[ADR-0004](../adr/0004-hosting-and-infrastructure.md) で
ローカル実行用の Dockerfile は**本番環境でも使える水準**とすると決めている。
一方、開発時にはホットリロードやデバッガなど本番に不要なものが要る。

同一ファイルに両方を詰め込むと、本番イメージに開発ツールが混入する。
マルチステージビルドで分ける。

### ARM での動作確認

[ADR-0004](../adr/0004-hosting-and-infrastructure.md) の
「検証が必要な事項」に挙げた **SudachiPy を含む全依存の ARM 動作検証**を、
この Issue の中で行う。

開発マシンが ARM（Apple Silicon）であれば自然に検証されるが、
x86 の場合は `--platform linux/arm64` でビルドして確認する。
ここで動かないことが分かれば、ホスティングの方針を早期に見直せる。

## 参考

- [ADR-0002: 言語・フレームワークの選定とサービス分割](../adr/0002-tech-stack.md)
- [ADR-0004: ホスティング先とインフラ構成](../adr/0004-hosting-and-infrastructure.md)
- [基本設計 01: システム構成](../design/basic/01-architecture.md)
- [docs/README.md「README に将来追記する項目」](../README.md#readme-に将来追記する項目)
