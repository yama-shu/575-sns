#!/usr/bin/env bash
# CI が計測したカバレッジを PR に1つのコメントとして投稿・更新する。
#
# 実行のたびに新しいコメントを足すと、PR が通知で埋まって読めなくなる。
# 目印（HTML コメント）を仕込んでおき、既存のコメントがあれば書き換える。
#
# 使い方:
#   GH_TOKEN=... PR_NUMBER=10 ./scripts/ci/coverage-comment.sh <アーティファクトのディレクトリ>

set -euo pipefail

ARTIFACT_DIR="${1:-artifacts}"
MARKER="<!-- coverage-report -->"

if [ -z "${PR_NUMBER:-}" ]; then
  echo "PR_NUMBER が設定されていません。PR 以外では実行しません。" >&2
  exit 0
fi

# ---- 目標値（NFR-05-04）----
target_of() {
  case "$1" in
    prosody) echo 100 ;;
    api)     echo 90 ;;
    web)     echo 60 ;;
    *)       echo 0 ;;
  esac
}

rows=""

# prosody: pytest-cov が出力する JSON から総合カバレッジを取り出す
prosody_json="${ARTIFACT_DIR}/coverage-prosody/coverage.json"
if [ -f "$prosody_json" ]; then
  pct=$(jq -r '.totals.percent_covered | . * 100 | round / 100' "$prosody_json")
  target=$(target_of prosody)
  mark=$(awk -v p="$pct" -v t="$target" 'BEGIN { print (p + 0 >= t + 0) ? "✅" : "⚠️" }')
  rows="${rows}| prosody | ${pct}% | ${target}% | ${mark} |"$'\n'
fi

# api: go tool cover の出力（total 行）から取り出す
api_out="${ARTIFACT_DIR}/coverage-api/coverage.out"
if [ -f "$api_out" ]; then
  if command -v go >/dev/null 2>&1; then
    pct=$(go tool cover -func="$api_out" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')
  else
    # go が無い環境では coverage.out を直接集計する
    pct=$(awk 'NR > 1 { total += $2; if ($3 > 0) covered += $2 }
               END { printf "%.1f", (total ? covered / total * 100 : 0) }' "$api_out")
  fi
  target=$(target_of api)
  mark=$(awk -v p="$pct" -v t="$target" 'BEGIN { print (p + 0 >= t + 0) ? "✅" : "⚠️" }')
  rows="${rows}| api | ${pct}% | ${target}% | ${mark} |"$'\n'
fi

if [ -z "$rows" ]; then
  echo "カバレッジのアーティファクトが見つかりませんでした。コメントは投稿しません。"
  exit 0
fi

body=$(cat <<EOF
${MARKER}
## カバレッジ

| サービス | 実測 | 目標 | |
| --- | ---: | ---: | :---: |
${rows}
目標は [NFR-05-04](../blob/main/docs/requirements/01-requirements.md#nfr-05-保守性運用性) にもとづく C1（分岐網羅）です。
**prosody は 100% を下回ると CI が失敗します。** 他のサービスは現時点では表示のみです。

<sub>変更のなかったサービスは実行されないため、表に出ません。</sub>
EOF
)

# 既存のコメントを探して、あれば更新する
existing=$(gh api "repos/${GITHUB_REPOSITORY}/issues/${PR_NUMBER}/comments" \
  --jq "map(select(.body | startswith(\"${MARKER}\"))) | .[0].id // empty")

if [ -n "$existing" ]; then
  gh api --method PATCH "repos/${GITHUB_REPOSITORY}/issues/comments/${existing}" \
    -f body="$body" > /dev/null
  echo "既存のコメント（id=${existing}）を更新しました。"
else
  gh api --method POST "repos/${GITHUB_REPOSITORY}/issues/${PR_NUMBER}/comments" \
    -f body="$body" > /dev/null
  echo "コメントを新規投稿しました。"
fi
