#!/usr/bin/env bash
# 結合テスト用のデータベースを用意する。
#
# 結合テスト（api/internal/infra/postgres）は reports / posts / users を
# **全件削除する**。開発用の DB に向けると手元のデータが消えるため、
# 別のデータベースを作り、そこにだけマイグレーションを適用する。
#
# 使い方:
#   ./scripts/testdb.sh            テスト用 DB を作り、マイグレーションを適用する
#   ./scripts/testdb.sh --drop     テスト用 DB を削除する
#
# 実行後に表示される DSN を API_TEST_DATABASE_URL に渡してテストを実行する。
#
# 前提: `docker compose up` で db が起動していること。

set -euo pipefail

cd "$(dirname "$0")/.."

# .env があれば読む。POSTGRES_USER などを既定値から変えている場合に合わせる。
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

USER_NAME="${POSTGRES_USER:-sns575}"
PASSWORD="${POSTGRES_PASSWORD:-local-dev-only}"
DEV_DB="${POSTGRES_DB:-sns575}"

# **名前の末尾を _test にする。** 結合テストは接続先の名前を確かめ、
# この規則に合わない DB では実行を拒否する（integration_test.go）。
TEST_DB="${DEV_DB}_test"

# コンテナの中から見た DSN（api コンテナで go test を動かすため）。
DSN="postgres://${USER_NAME}:${PASSWORD}@db:5432/${TEST_DB}?sslmode=disable"

psql_db() { # $1=接続先DB $2...=psql の引数
  local db="$1"; shift
  docker compose exec -T db psql -v ON_ERROR_STOP=1 -U "${USER_NAME}" -d "$db" "$@"
}

if [ "${1:-}" = "--drop" ]; then
  psql_db "$DEV_DB" -c "DROP DATABASE IF EXISTS ${TEST_DB}"
  echo "${TEST_DB} を削除しました。"
  exit 0
fi

if [ -n "${1:-}" ]; then
  echo "不明な引数: $1（--drop のみ受け付けます）" >&2
  exit 2
fi

# CREATE DATABASE は IF NOT EXISTS を持たない。存在を先に確かめる。
exists=$(psql_db "$DEV_DB" -tAc "SELECT 1 FROM pg_database WHERE datname = '${TEST_DB}'")
if [ "$exists" = "1" ]; then
  echo "${TEST_DB} は既にあります。"
else
  psql_db "$DEV_DB" -c "CREATE DATABASE ${TEST_DB}"
  echo "${TEST_DB} を作成しました。"
fi

# **マイグレーションはテスト用 DB にだけ適用する。**
# migrate サービスは API_DATABASE_URL を読むため、ここで上書きする。
docker compose run --rm -e API_DATABASE_URL="$DSN" migrate up

cat <<EOS

テスト用の DB が整いました。結合テストは次のように実行します。

  docker compose exec -e API_TEST_DATABASE_URL='${DSN}' \\
    api go test ./internal/infra/postgres/... -v

EOS
