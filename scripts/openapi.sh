#!/usr/bin/env bash
# OpenAPI 定義を扱う。
#
# 3サービスは言語が違い、型定義を共有できない（ADR-0002）。
# その代わりに **API の契約を OpenAPI 定義としてリポジトリで管理する**
# （基本設計 05 §6）。
#
#   prosody : FastAPI が型ヒントから生成する → 書き出してコミットする
#   api     : 生成元が無いため手で書く       → 妥当性を検証する
#   web     : api の定義から型を生成する     → 生成物をコミットする
#
# 使い方:
#   ./scripts/openapi.sh          書き出し・検証・型生成をすべて行う
#   ./scripts/openapi.sh --check  最新かどうかを確かめる（差分があれば失敗）
#
# prosody はサーバを起動せずに生成するため、辞書のロードは発生しない。

set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

PROSODY_SPEC="prosody/openapi.json"
API_SPEC="api/openapi.yaml"
WEB_TYPES="web/src/lib/api/schema.d.ts"

UV_IMAGE="ghcr.io/astral-sh/uv:python3.13-bookworm-slim"
NODE_IMAGE="node:24-alpine"
REDOCLY="@redocly/cli@1.34.1"
OPENAPI_TS="openapi-typescript@7.9.1"

CHECK=0
[ "${1:-}" = "--check" ] && CHECK=1

# ---------------------------------------------------------------------------
# prosody: FastAPI から書き出す
# ---------------------------------------------------------------------------
generate_prosody() {
  docker run --rm -v "${ROOT}/prosody:/app" -w /app "$UV_IMAGE" sh -c '
    uv sync --frozen --quiet >/dev/null 2>&1
    uv run python -c "
import json
from prosody.main import app
print(json.dumps(app.openapi(), ensure_ascii=False, indent=2, sort_keys=True))
"'
}

# ---------------------------------------------------------------------------
# api: 手で書いた定義を検証する
#
# **生成できないため、妥当性の検査が唯一の機械的な歯止めになる。**
# 定義とサーバーの実際の応答が一致するかは、
# api/internal/handler/openapi_test.go で別途確かめる。
# ---------------------------------------------------------------------------
lint_api() {
  docker run --rm -v "${ROOT}/api:/spec" -w /spec "$NODE_IMAGE" \
    npx --yes "$REDOCLY" lint openapi.yaml
}

# ---------------------------------------------------------------------------
# web: api の定義から型を生成する
#
# **型だけを生成し、HTTP クライアントは生成しない。** 生成したクライアントは
# Cookie の扱いやエラー処理を独自に持ち込み、詳細設計 03 のエラー設計と
# 食い違いやすい。
# ---------------------------------------------------------------------------
generate_web_types() {
  docker run --rm -v "${ROOT}:/work" -w /work "$NODE_IMAGE" \
    npx --yes "$OPENAPI_TS" api/openapi.yaml
}

if [ "$CHECK" -eq 1 ]; then
  failed=0

  if [ ! -f "$PROSODY_SPEC" ]; then
    echo "$PROSODY_SPEC がありません。./scripts/openapi.sh を実行してください。" >&2
    exit 1
  fi
  if ! generate_prosody | diff -u "$PROSODY_SPEC" - > /tmp/575-openapi-prosody.diff; then
    echo "$PROSODY_SPEC が最新ではありません。./scripts/openapi.sh を実行してください。" >&2
    head -40 /tmp/575-openapi-prosody.diff >&2
    failed=1
  else
    echo "$PROSODY_SPEC は最新です。"
  fi

  if ! lint_api > /tmp/575-openapi-lint.log 2>&1; then
    echo "$API_SPEC の検証に失敗しました。" >&2
    tail -30 /tmp/575-openapi-lint.log >&2
    failed=1
  else
    echo "$API_SPEC は妥当です。"
  fi

  if [ ! -f "$WEB_TYPES" ]; then
    echo "$WEB_TYPES がありません。./scripts/openapi.sh を実行してください。" >&2
    exit 1
  fi
  if ! generate_web_types | diff -u "$WEB_TYPES" - > /tmp/575-openapi-web.diff; then
    echo "$WEB_TYPES が最新ではありません。./scripts/openapi.sh を実行して差分をコミットしてください。" >&2
    head -40 /tmp/575-openapi-web.diff >&2
    failed=1
  else
    echo "$WEB_TYPES は最新です。"
  fi

  exit "$failed"
fi

generate_prosody > "$PROSODY_SPEC"
echo "$PROSODY_SPEC を書き出しました（$(wc -l < "$PROSODY_SPEC" | tr -d ' ') 行）。"

lint_api
echo "$API_SPEC を検証しました。"

mkdir -p "$(dirname "$WEB_TYPES")"
generate_web_types > "$WEB_TYPES"
echo "$WEB_TYPES を生成しました（$(wc -l < "$WEB_TYPES" | tr -d ' ') 行）。"
