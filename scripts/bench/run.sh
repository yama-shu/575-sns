#!/usr/bin/env bash
# prosody の性能を測定する（#9 / NFR-01-01）。
#
# 測定を再現できるよう、次をすべてスクリプト化する。
#   1. 本番相当のイメージで prosody を起動する
#   2. 入力データ 250 件（すべて異なる）を生成する
#   3. 負荷をかけ、P50 / P95 / P99 / エラー率を得る
#   4. 段階ごとの内訳とメモリ使用量、辞書のロード時間を測る
#   5. 測定環境を記録する
#
# 使い方:
#   ./scripts/bench/run.sh                  既定（同時50接続・30秒）
#   ./scripts/bench/run.sh --duration 60s
#   ./scripts/bench/run.sh --connections 100
#   ./scripts/bench/run.sh --workers 4      prosody のワーカー数を変える
#
# 結果は tmp/bench/ に残る。

set -euo pipefail

cd "$(dirname "$0")/../.."
ROOT="$(pwd)"

DURATION="30s"
CONNECTIONS=50
WORKERS=1
OUT_DIR="${ROOT}/tmp/bench"
NETWORK="575_default"

UV_IMAGE="ghcr.io/astral-sh/uv:python3.13-bookworm-slim"
# k6 を選ぶ理由:
#   - arm64 のイメージがある。本番も開発機も ARM のため、
#     エミュレーションで負荷生成側が遅くなると測定が歪む。
#   - リクエストごとに**別の本文**を送れる。同じ文を繰り返すと
#     辞書の内部キャッシュが効いて実態より速く見える。
K6_IMAGE="grafana/k6:latest"

while [ $# -gt 0 ]; do
  case "$1" in
    --duration) DURATION="$2"; shift 2 ;;
    --connections) CONNECTIONS="$2"; shift 2 ;;
    --workers) WORKERS="$2"; shift 2 ;;
    *) echo "不明な引数: $1" >&2; exit 2 ;;
  esac
done

mkdir -p "$OUT_DIR"

echo "=== 1. prosody を本番相当で起動する（workers=${WORKERS}） ==="
PROSODY_WORKERS="$WORKERS" docker compose -f compose.yaml up -d --build >/dev/null 2>&1
until curl -sf --max-time 3 localhost:8080/readyz >/dev/null 2>&1; do sleep 2; done
echo "起動しました。"

echo
echo "=== 2. 入力データを生成する（250件・すべて異なる） ==="
docker run --rm \
  -v "${ROOT}/prosody:/app" -v "${ROOT}/scripts:/scripts" -w /app \
  "$UV_IMAGE" sh -c 'uv sync --frozen --quiet >/dev/null 2>&1
    uv run python /scripts/bench/generate_inputs.py' \
  > "${OUT_DIR}/inputs.json"
cp "${OUT_DIR}/inputs.json" "${ROOT}/scripts/bench/inputs.json"

echo
echo "=== 3. 負荷をかける（同時${CONNECTIONS}接続 / ${DURATION}） ==="
docker run --rm \
  -v "${ROOT}/scripts/bench:/bench" -w /bench \
  --network "$NETWORK" \
  -e TARGET_URL="http://prosody:8000/v1/analyze" \
  -e CONNECTIONS="$CONNECTIONS" \
  -e DURATION="$DURATION" \
  "$K6_IMAGE" run --summary-export=/bench/summary.json load.js \
  2>&1 | tee "${OUT_DIR}/k6.log" || true
mv "${ROOT}/scripts/bench/summary.json" "${OUT_DIR}/summary.json" 2>/dev/null || true
rm -f "${ROOT}/scripts/bench/inputs.json"

echo
echo "=== 4. 段階ごとの内訳と辞書のロード時間を測る ==="
docker run --rm \
  -v "${ROOT}/prosody:/app" -v "${ROOT}/scripts:/scripts" -w /app \
  "$UV_IMAGE" sh -c 'uv run python /scripts/bench/stages.py' \
  > "${OUT_DIR}/stages.json"

echo
echo "=== 5. メモリ使用量 ==="
docker stats --no-stream --format '{{.Name}}\t{{.MemUsage}}\t{{.CPUPerc}}' \
  | tee "${OUT_DIR}/memory.txt"

echo
echo "=== 測定環境 ==="
{
  echo "date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "docker: $(docker --version)"
  echo "compose: $(docker compose version --short)"
  docker info --format 'arch={{.Architecture}} cpu={{.NCPU}} mem_bytes={{.MemTotal}}'
  echo "prosody_image: $(docker compose -f compose.yaml images prosody --format json | jq -r '.[0].Tag' 2>/dev/null || echo runtime)"
  echo "prosody_workers=${WORKERS} connections=${CONNECTIONS} duration=${DURATION}"
  echo "commit: $(git rev-parse --short HEAD)"
} | tee "${OUT_DIR}/environment.txt"

echo
echo "結果は ${OUT_DIR}/ に残っています。"
