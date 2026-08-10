-- タイムラインの実行計画を測定するためのデータを投入する（#40）
--
-- 既存のデータはすべて消す。測定は決まった条件で再現できる必要がある。
--
-- **投入後は VACUUM ANALYZE を実行する。** ANALYZE だけでは統計しか更新されず、
-- 可視性マップが整わないため Index Only Scan が選ばれない。
-- 投入直後と autovacuum 後とで計画が変わり、測定値が再現しなくなる。
--
-- 規模は基本設計 03 §3 の「posts 10万行以上」に合わせる。
-- 分布を偏らせるのは、全員が同じ数の投稿を持つデータだと
-- 実際とは違う計画が選ばれうるためである。

\set ON_ERROR_STOP on

BEGIN;

TRUNCATE reports, likes, blocks, follows, posts, sessions, users RESTART IDENTITY CASCADE;

-- ---------------------------------------------------------------------------
-- 利用者 1,000 人
--
-- パスワードハッシュは測定に使わないため固定値でよい。
-- bcrypt を 1,000 回走らせると投入だけで数十秒かかる。
-- ---------------------------------------------------------------------------
INSERT INTO users (handle, email, password_hash, display_name, status)
SELECT
    'user' || g,
    'user' || g || '@example.com',
    '$2a$04$0123456789012345678901234567890123456789012345678901',
    '利用者' || g,
    'active'
FROM generate_series(1, 1000) g;

-- ---------------------------------------------------------------------------
-- 投稿 120,000 件
--
-- 投稿数を利用者ごとに偏らせる。
--   上位 1%（10人）  : 1人あたり約 2,000 件
--   中位 19%（190人）: 1人あたり約 400 件
--   下位 80%（800人）: 1人あたり約 30 件
--
-- 公開範囲は 9 : 1 で public : followers とする。
-- ---------------------------------------------------------------------------
INSERT INTO posts (author_id, body, reading, verdict,
                   break1, break2, mora_kami, mora_naka, mora_shimo,
                   visibility, status, like_count, created_at)
SELECT
    u.id,
    '今日もまた会議のための会議かな',
    'キョウモマタカイギノタメノカイギカナ',
    'teikei',
    5, 11, 5, 7, 5,
    CASE WHEN random() < 0.1 THEN 'followers' ELSE 'public' END,
    'published',
    0,
    now() - (random() * interval '365 days')
FROM users u
CROSS JOIN LATERAL generate_series(
    1,
    CASE
        WHEN u.id <= 10  THEN 2000
        WHEN u.id <= 200 THEN 400
        ELSE 30
    END
) g
-- **投稿 ID と著者 ID を相関させない。**
-- 著者ごとにまとめて挿入すると BIGSERIAL が著者順に並び、
-- 「新しい投稿ほど特定の著者に偏る」という現実には無い分布になる。
-- タイムラインは id 降順に読むため、この相関がそのまま実行計画を歪める。
ORDER BY random();

-- 論理削除された投稿を 5% 作る。
-- 部分インデックスが削除済みの蓄積に強いことを確かめるため、
-- 「削除済みが混ざっている」状態にする（基本設計 03 §3）。
UPDATE posts
SET status = 'deleted', deleted_at = now()
WHERE id % 20 = 0;

-- ---------------------------------------------------------------------------
-- フォロー
--
-- 利用者1（測定の主体）が 150 人をフォローする。
-- ADR-0005 が「1ユーザーあたりの平均フォロー数」を監視指標としており、
-- JOIN の重さに直結する。
--
-- **150 に外部の根拠はない。** 575 はまだ公開しておらず、実際の分布が無い。
-- 「多めに見積もる」という方針で選んだ値である。
-- 公開後に実データを見て置き換える。
--
-- フォロー数そのものより、フォロー先の投稿が全体に占める割合のほうが
-- 実行計画に効く（docs/perf/0002 §6-B）。測定では割合の違う利用者を
-- 複数用意して比べる。
--
-- 他の利用者にもフォロー関係を作る。利用者1だけが持つ状態だと、
-- follows の統計が偏ってプランナの判断が実際と変わる。
-- ---------------------------------------------------------------------------
INSERT INTO follows (follower_id, followee_id)
SELECT 1, g FROM generate_series(2, 151) g;

INSERT INTO follows (follower_id, followee_id)
SELECT f.id, t.id
FROM users f
CROSS JOIN LATERAL (
    SELECT id FROM users
    WHERE id <> f.id
    ORDER BY id
    OFFSET (f.id * 7) % 900
    LIMIT 50
) t
WHERE f.id BETWEEN 2 AND 1000
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- ブロック
--
-- 0 件にしない。0 件だとプランナが Seq Scan を選び、
-- インデックスを使えるかどうかを判断できない（#38 で判明）。
--
-- 利用者1 が 20 人をブロックし、30 人からブロックされている状態にする。
-- 双方向の除外がどちらの向きでも効くかを見るため。
-- ---------------------------------------------------------------------------
INSERT INTO blocks (blocker_id, blocked_id)
SELECT 1, g FROM generate_series(800, 819) g;

INSERT INTO blocks (blocker_id, blocked_id)
SELECT g, 1 FROM generate_series(850, 879) g;

INSERT INTO blocks (blocker_id, blocked_id)
SELECT b.id, ((b.id * 13) % 1000) + 1
FROM generate_series(2, 500) b(id)
WHERE ((b.id * 13) % 1000) + 1 <> b.id
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- いいね
--
-- liked_by_me を EXISTS で取っているため、likes の規模も計画に効く。
-- ---------------------------------------------------------------------------
INSERT INTO likes (user_id, post_id)
SELECT 1, p.id
FROM posts p
WHERE p.id % 37 = 0
LIMIT 3000;

INSERT INTO likes (user_id, post_id)
SELECT ((p.id * 7) % 999) + 2, p.id
FROM posts p
WHERE p.id % 11 = 0
ON CONFLICT DO NOTHING;

COMMIT;

-- 統計を更新し、可視性マップを整える。
--
-- **ANALYZE だけでは足りない。** 統計は更新されるが可視性マップは整わず、
-- Index Only Scan が選べないままヒープを読む計画になる。
-- 投入直後に測ると、autovacuum が走ったあとの計画と食い違う（#40 で判明）。
--
-- VACUUM は本番でも autovacuum が担う処理であり、
-- 定常状態に近い条件で測るために先に実行する。
VACUUM ANALYZE users, posts, follows, blocks, likes;

SELECT
    (SELECT count(*) FROM users)   AS users,
    (SELECT count(*) FROM posts)   AS posts,
    (SELECT count(*) FROM posts WHERE status = 'published' AND visibility = 'public') AS public_posts,
    (SELECT count(*) FROM follows) AS follows,
    (SELECT count(*) FROM blocks)  AS blocks,
    (SELECT count(*) FROM likes)   AS likes;
