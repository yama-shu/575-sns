# api（Go / Echo）— 業務ロジックとデータの永続化
#
# ステージ構成
#   deps    : 依存モジュールだけを先に取得し、ソース変更でキャッシュが壊れないようにする
#   dev     : 開発用。air によるホットリロードを含む（本番には含めない）
#   build   : 本番用のバイナリをビルドする
#   runtime : 本番用。バイナリ1つだけを置く最小構成

ARG GO_VERSION=1.26

# ---------- deps ----------
FROM golang:${GO_VERSION}-bookworm AS deps
WORKDIR /app
# go.mod / go.sum だけを先に置く。ソースを変えても依存の再取得が走らない。
COPY go.mod go.sum ./
# ここでキャッシュマウントを使ってはならない。キャッシュマウントの中身はイメージ層に
# 残らないため、dev コンテナを起動するたびに air が全モジュールを再ダウンロードする。
# 素の RUN にすることでモジュールがイメージに焼き込まれ、起動が即座になる。
# go.mod / go.sum が変わらない限り、この層は Docker のレイヤキャッシュが効く。
RUN go mod download

# ---------- dev（ホットリロード付き） ----------
FROM deps AS dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/air-verse/air@v1.67.4
EXPOSE 8080
# ソースは compose がバインドマウントする
CMD ["air", "-c", ".air.toml"]

# ---------- build ----------
FROM deps AS build
COPY . .
# CGO を無効にして静的リンクする。runtime に libc を置かずに済む。
# /go/pkg/mod をキャッシュマウントで覆うと deps 層のモジュールが隠れるため、
# ここではコンパイル結果のキャッシュだけを使う。
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---------- runtime（本番でも使える水準） ----------
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/api /api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
