#!/usr/bin/env bash
#
# タイムラインの実行計画を測定する（#40）。
#
# 基本設計 03 §3 が実装時の EXPLAIN ANALYZE の取得を必須としている。
# データの投入から測定まで、この1本で再現できるようにする。
#
#   ./scripts/bench/timeline_explain.sh            投入してから測定する
#   ./scripts/bench/timeline_explain.sh --no-seed  すでにあるデータで測定する
#
# 出力はそのまま docs/perf/0002-timeline-explain.md に貼れる形にする。

set -euo pipefail

cd "$(dirname "$0")/../.."

SEED=1
[[ "${1:-}" == "--no-seed" ]] && SEED=0

psql() { docker compose exec -T db psql -U "${POSTGRES_USER:-sns575}" -d "${POSTGRES_DB:-sns575}" "$@"; }

# 測定の主体。seed_timeline.sql が 150 フォロー・20 ブロック・
# 30 被ブロックの状態を作る利用者。
ME=1

if [[ $SEED -eq 1 ]]; then
  echo "== データを投入する =="
  psql -v ON_ERROR_STOP=1 -f - < scripts/bench/seed_timeline.sql | tail -4
  echo
fi

echo "== 測定環境 =="
psql -t -c "SELECT 'PostgreSQL ' || current_setting('server_version');"
psql -t -c "SELECT '  shared_buffers = ' || current_setting('shared_buffers')
                || ' / work_mem = ' || current_setting('work_mem')
                || ' / effective_cache_size = ' || current_setting('effective_cache_size');"
echo

# explain は EXPLAIN (ANALYZE, BUFFERS) を実行する。
# 1回目はキャッシュが冷えているため、2回実行して2回目を採る。
explain() {
  local title="$1" query="$2"
  echo "--- ${title} ---"
  psql -q -c "EXPLAIN (ANALYZE, BUFFERS) ${query}" > /dev/null
  psql -c "EXPLAIN (ANALYZE, BUFFERS) ${query}"
  echo
}

PUBLIC_TIMELINE="
SELECT p.id, p.body,
       EXISTS (SELECT 1 FROM likes l WHERE l.post_id = p.id AND l.user_id = ${ME}) AS liked_by_me
FROM posts p
JOIN users u ON u.id = p.author_id
WHERE p.status = 'published'
  AND p.visibility = 'public'
  AND (CURSOR_PLACEHOLDER)
  AND NOT EXISTS (
        SELECT 1 FROM blocks b
        WHERE (b.blocker_id = ${ME} AND b.blocked_id = p.author_id)
           OR (b.blocker_id = p.author_id AND b.blocked_id = ${ME}))
ORDER BY p.id DESC
LIMIT 20"

HOME_TIMELINE="
SELECT p.id, p.body,
       EXISTS (SELECT 1 FROM likes l WHERE l.post_id = p.id AND l.user_id = ${ME}) AS liked_by_me
FROM posts p
JOIN users u ON u.id = p.author_id
JOIN follows f ON f.followee_id = p.author_id AND f.follower_id = ${ME}
WHERE p.status = 'published'
  AND (CURSOR_PLACEHOLDER)
  AND NOT EXISTS (
        SELECT 1 FROM blocks b
        WHERE (b.blocker_id = ${ME} AND b.blocked_id = p.author_id)
           OR (b.blocker_id = p.author_id AND b.blocked_id = ${ME}))
ORDER BY p.id DESC
LIMIT 20"

with_cursor() { echo "${1//(CURSOR_PLACEHOLDER)/$2}"; }

echo "== 1. 全体タイムライン（先頭ページ） =="
explain "全体タイムライン / cursor なし" "$(with_cursor "$PUBLIC_TIMELINE" "TRUE")"

echo "== 2. フォロー中タイムライン（先頭ページ） =="
explain "フォロー中タイムライン / cursor なし" "$(with_cursor "$HOME_TIMELINE" "TRUE")"

echo "== 3. カーソルの深さによる違い =="
for cursor in 119000 60000 1000; do
  explain "全体タイムライン / cursor = ${cursor}" \
    "$(with_cursor "$PUBLIC_TIMELINE" "p.id < ${cursor}")"
done

echo "== 4. OFFSET 方式との比較 =="
for offset in 0 1000 20000 100000; do
  echo "--- OFFSET ${offset} ---"
  psql -q -c "EXPLAIN (ANALYZE, BUFFERS)
    SELECT p.id FROM posts p
    WHERE p.status = 'published' AND p.visibility = 'public'
    ORDER BY p.id DESC LIMIT 20 OFFSET ${offset}" > /dev/null
  psql -c "EXPLAIN (ANALYZE, BUFFERS)
    SELECT p.id FROM posts p
    WHERE p.status = 'published' AND p.visibility = 'public'
    ORDER BY p.id DESC LIMIT 20 OFFSET ${offset}" | grep -E "Execution Time|Buffers: shared|Limit"
  echo
done

echo "== 5. カーソル方式の同条件（比較用） =="
for cursor in 120001 119000 100000 20001; do
  echo "--- cursor = ${cursor} ---"
  psql -q -c "EXPLAIN (ANALYZE, BUFFERS)
    SELECT p.id FROM posts p
    WHERE p.status = 'published' AND p.visibility = 'public' AND p.id < ${cursor}
    ORDER BY p.id DESC LIMIT 20" > /dev/null
  psql -c "EXPLAIN (ANALYZE, BUFFERS)
    SELECT p.id FROM posts p
    WHERE p.status = 'published' AND p.visibility = 'public' AND p.id < ${cursor}
    ORDER BY p.id DESC LIMIT 20" | grep -E "Execution Time|Buffers: shared|Limit"
  echo
done
