#!/usr/bin/env bash
# prosody の OpenAPI 定義を書き出す。
#
# FastAPI は型ヒントから定義を自動生成するが、そのままではリポジトリに
# 残らず、差分をレビューできない。api（Go）と web（TypeScript）は
# この定義を契約として型を生成するため、変更に気づけないと
# 「api が返すもの」と「web が期待するもの」がずれる（ADR-0002）。
#
# 使い方:
#   ./scripts/openapi.sh          定義を書き出す
#   ./scripts/openapi.sh --check  最新かどうかを確かめる（差分があれば失敗）
#
# サーバを起動せずに生成するため、辞書のロードは発生しない。

set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"
OUTPUT="prosody/openapi.json"
UV_IMAGE="ghcr.io/astral-sh/uv:python3.13-bookworm-slim"

generate() {
  docker run --rm -v "${ROOT}/prosody:/app" -w /app "$UV_IMAGE" sh -c '
    uv sync --frozen --quiet >/dev/null 2>&1
    uv run python -c "
import json
from prosody.main import app
print(json.dumps(app.openapi(), ensure_ascii=False, indent=2, sort_keys=True))
"'
}

if [ "${1:-}" = "--check" ]; then
  if [ ! -f "$OUTPUT" ]; then
    echo "$OUTPUT がありません。./scripts/openapi.sh を実行してください。" >&2
    exit 1
  fi
  if ! generate | diff -u "$OUTPUT" - > /tmp/575-openapi.diff; then
    echo "$OUTPUT が最新ではありません。./scripts/openapi.sh を実行して差分をコミットしてください。" >&2
    echo >&2
    head -40 /tmp/575-openapi.diff >&2
    exit 1
  fi
  echo "$OUTPUT は最新です。"
  exit 0
fi

generate > "$OUTPUT"
echo "$OUTPUT を書き出しました（$(wc -l < "$OUTPUT" | tr -d ' ') 行）。"
