# prosody（Python / FastAPI）— 五七五判定エンジン
#
# ステージ構成
#   deps    : 本番用の依存だけを解決した仮想環境をつくる
#   dev     : 開発用。dev 依存とホットリロードを含む（本番には含めない）
#   runtime : 本番用。ビルドツールも dev 依存も含まない最小構成
#
# ADR-0003 のとおり SudachiPy は C++ のビルド依存がなく、
# aarch64 の wheel が提供されているため、ARM でもコンパイラを必要としない。

ARG PYTHON_VERSION=3.13

# ---------- base ----------
FROM ghcr.io/astral-sh/uv:python${PYTHON_VERSION}-bookworm-slim AS base
ENV UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1
WORKDIR /app

# ---------- deps（本番用の依存のみ） ----------
FROM base AS deps
COPY pyproject.toml uv.lock ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-install-project --no-dev

# ---------- dev（ホットリロード付き） ----------
FROM base AS dev
COPY pyproject.toml uv.lock ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-install-project
ENV PATH="/app/.venv/bin:${PATH}" \
    PYTHONPATH=/app/src
EXPOSE 8000
# ソースは compose がバインドマウントする。--reload で変更を即座に反映する。
CMD ["uvicorn", "prosody.main:app", \
     "--host", "0.0.0.0", "--port", "8000", \
     "--reload", "--reload-dir", "/app/src"]

# ---------- runtime（本番でも使える水準） ----------
FROM python:${PYTHON_VERSION}-slim-bookworm AS runtime
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PATH="/app/.venv/bin:${PATH}" \
    PYTHONPATH=/app/src
WORKDIR /app

# root で動かさない
RUN groupadd --system --gid 1001 prosody \
 && useradd --system --uid 1001 --gid prosody prosody

COPY --from=deps --chown=prosody:prosody /app/.venv /app/.venv
COPY --chown=prosody:prosody src ./src

USER prosody
EXPOSE 8000

# 判定は CPU バウンドのため、プロセス数はコンテナに割り当てた CPU に合わせて
# 環境変数 PROSODY_WORKERS で上書きする（既定は 1 プロセス）。
ENV PROSODY_WORKERS=1
CMD ["sh", "-c", "exec uvicorn prosody.main:app --host 0.0.0.0 --port 8000 --workers ${PROSODY_WORKERS}"]
