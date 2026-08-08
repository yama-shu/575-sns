#!/usr/bin/env bash
# CI と同じ検査を手元で実行する。
#
# CI に push してから lint で落ちるのを待つのは時間の無駄なので、
# 同じ内容をローカルで先に流せるようにしている。
# 使うイメージとバージョンは CI（.github/workflows/ci.yml）と揃えてある。
#
# 使い方:
#   ./scripts/check.sh              すべてのサービスを検査する
#   ./scripts/check.sh prosody      特定のサービスだけ検査する
#   ./scripts/check.sh api web
#
# 前提: Docker が動いていること。各言語の処理系をホストに入れる必要はない。

set -uo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

UV_IMAGE="ghcr.io/astral-sh/uv:python3.13-bookworm-slim"
GO_IMAGE="golangci/golangci-lint:v2.12.2"
NODE_IMAGE="node:24-bookworm-slim"

failed=()

section() { printf '\n\033[1m=== %s ===\033[0m\n' "$1"; }
ok()      { printf '  \033[32m✓\033[0m %s\n' "$1"; }
ng()      { printf '  \033[31m✗\033[0m %s\n' "$1"; failed+=("$1"); }

run_step() { # $1=表示名 $2...=コマンド
  local label="$1"; shift
  if "$@" > /tmp/575-check.log 2>&1; then
    ok "$label"
  else
    ng "$label"
    sed 's/^/      /' /tmp/575-check.log | tail -25
  fi
}

check_prosody() {
  section "prosody（Python / FastAPI）"
  local d="docker run --rm -v ${ROOT}/prosody:/app -w /app ${UV_IMAGE}"
  $d uv sync --frozen --quiet > /dev/null 2>&1 || { ng "依存の解決"; return; }
  run_step "lint (ruff)"        $d uv run ruff check .
  run_step "整形 (ruff format)" $d uv run ruff format --check .
  run_step "型検査 (mypy)"      $d uv run mypy src
  run_step "単体テスト（C1 100%）" $d uv run pytest --cov-fail-under=100 -q
}

check_api() {
  section "api（Go / Echo）"
  local d="docker run --rm -v ${ROOT}/api:/app -w /app ${GO_IMAGE}"
  run_step "整形 (gofmt)"          $d sh -c 'test -z "$(gofmt -l .)"'
  run_step "go vet"                $d go vet ./...
  run_step "lint (golangci-lint)"  $d golangci-lint run ./...
  run_step "単体テスト"            $d go test ./... -cover
}

check_web() {
  section "web（TypeScript / Next.js）"
  # node_modules はホスト（macOS）のものを使わず、コンテナ内で入れ直す
  local d="docker run --rm -v ${ROOT}/web:/app -w /app ${NODE_IMAGE}"
  run_step "lint (eslint)"   $d npm run lint
  run_step "型検査 (tsc)"    $d npm run typecheck
  run_step "ビルド"          $d npm run build
}

targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
  targets=(prosody api web)
fi

for t in "${targets[@]}"; do
  case "$t" in
    prosody) check_prosody ;;
    api)     check_api ;;
    web)     check_web ;;
    *) echo "不明なサービス: $t（prosody / api / web のいずれか）" >&2; exit 2 ;;
  esac
done

echo
if [ ${#failed[@]} -eq 0 ]; then
  printf '\033[32mすべての検査を通過しました。\033[0m\n'
  exit 0
fi
printf '\033[31m%d 件の検査に失敗しました:\033[0m\n' "${#failed[@]}"
printf '  - %s\n' "${failed[@]}"
exit 1
