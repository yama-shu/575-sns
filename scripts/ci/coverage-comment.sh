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

# ---- 目標値（詳細設計 04 のカバレッジ目標）----
#
# **api は層ごとに目標が違う。** モジュール全体の目標は定めていないため、
# 全体の値を目標と比べない（#95）。
target_of() {
  case "$1" in
    prosody)     echo 100 ;;
    api-usecase) echo 90 ;;
    api-infra)   echo 70 ;;
    web)         echo 60 ;;
    *)           echo 0 ;;
  esac
}

# 実測と目標を1行にする。
row() { # $1=表示名 $2=実測 $3=目標
  local mark
  mark=$(awk -v p="$2" -v t="$3" 'BEGIN { print (p + 0 >= t + 0) ? "✅" : "⚠️" }')
  printf '| %s | %s%% | %s%% | %s |\n' "$1" "$2" "$3" "$mark"
}

rows=""

# prosody: pytest-cov が出力する JSON から総合カバレッジを取り出す
prosody_json="${ARTIFACT_DIR}/coverage-prosody/coverage.json"
if [ -f "$prosody_json" ]; then
  pct=$(jq -r '.totals.percent_covered | . * 100 | round / 100' "$prosody_json")
  rows="${rows}$(row "prosody（分岐網羅）" "$pct" "$(target_of prosody)")"$'\n'
fi

# api: カバレッジプロファイルを直接集計する
#
# `go tool cover -func` を使わないのは、このスクリプトがリポジトリルートから
# 実行され、そこに go.mod が無いため。cover はモジュール解決を要求して失敗する。
# プロファイルの形式は「位置 文の数 実行回数」であり、集計に Go は要らない。
#
# **層ごとに集計する。** 目標が層ごとに定められており、全体の値と比べても
# 達成・未達を判断できない。位置の文字列で層を切り分ける。
api_out="${ARTIFACT_DIR}/coverage-api/coverage.out"

api_pct() { # $1=位置に含まれる文字列（空なら全体）
  awk -v frag="$1" 'NR > 1 && (frag == "" || index($1, frag)) {
                      total += $2; if ($3 > 0) covered += $2
                    }
                    END { printf "%.1f", (total ? covered / total * 100 : 0) }' "$api_out"
}

if [ -f "$api_out" ]; then
  rows="${rows}$(row "api の usecase 層" "$(api_pct /internal/usecase/)" "$(target_of api-usecase)")"$'\n'
  rows="${rows}$(row "api の infra 層" "$(api_pct /internal/infra/)" "$(target_of api-infra)")"$'\n'
  # 全体は参考値。目標を定めていないため、達成の記号を付けない。
  rows="${rows}| api 全体（参考） | $(api_pct '')% | — | |"$'\n'
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
目標は [詳細設計 04](../blob/main/docs/design/detail/04-test-design.md#カバレッジ目標)（[NFR-05-04](../blob/main/docs/requirements/01-requirements.md#nfr-05-保守性運用性)）にもとづきます。

**api は層ごとに目標が違います。** 全体の値はテストを書いていないパッケージ（\`cmd/*\` など）を
分母に含むため、参考として表示するだけで目標とは比べません。

指標は測定器によって異なります。prosody は \`--cov-branch\` による**分岐網羅**、
api は Go の \`cover\` による**文網羅**です（Go は分岐網羅を測れません）。

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
