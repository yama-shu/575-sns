-- フォロワー一覧（S-06）のためのインデックス。
--
-- 既存の follows_followee_id_idx は (followee_id) だけであり、
-- 「followee_id で絞って follower_id の降順で並べる」問い合わせに順序を与えない。
-- プランナは順序を得るために follows_pkey (follower_id, followee_id) を
-- 逆順に走査し、followee_id を条件として当てる計画を選ぶ。
--
-- 実測（PostgreSQL 18・follows 4,003 行・フォロワー3人の利用者）:
--
--     索引なし: follows の走査 25 buffers
--     索引あり: follows の走査  6 buffers
--
-- 絞り込みと順序を1つの索引で満たせるようにする。
CREATE INDEX follows_followers_list_idx ON follows (followee_id, follower_id DESC);

-- 既存の follows_followee_id_idx は上の索引の先頭列と重なるため不要になる。
-- 索引が増えるほど INSERT / DELETE のコストが上がるため、重複は残さない。
DROP INDEX IF EXISTS follows_followee_id_idx;
