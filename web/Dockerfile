# web（TypeScript / Next.js）— 画面描画と SSR
#
# ステージ構成
#   deps    : 依存だけを先に入れ、ソース変更でキャッシュが壊れないようにする
#   dev     : 開発用。Next.js の dev サーバでホットリロードする
#   build   : 本番用にビルドし、standalone 出力を得る
#   runtime : 本番用。standalone 出力だけを置く最小構成

ARG NODE_VERSION=24

# ---------- deps ----------
FROM node:${NODE_VERSION}-bookworm-slim AS deps
WORKDIR /app
# package.json / package-lock.json だけを先に置く。ソースを変えても再インストールが走らない。
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

# ---------- dev（ホットリロード付き） ----------
FROM deps AS dev
ENV NEXT_TELEMETRY_DISABLED=1
EXPOSE 3000
# ソースは compose がバインドマウントする
CMD ["npm", "run", "dev"]

# ---------- build ----------
FROM deps AS build
ENV NEXT_TELEMETRY_DISABLED=1
COPY . .
RUN npm run build

# ---------- runtime（本番でも使える水準） ----------
FROM node:${NODE_VERSION}-bookworm-slim AS runtime
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0
WORKDIR /app

# node イメージには uid 1000 の node ユーザが既にいる。root で動かさない。
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static

USER node
EXPOSE 3000
CMD ["node", "server.js"]
