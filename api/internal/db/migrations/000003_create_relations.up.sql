-- likes / follows / blocks
--
-- いずれも中間テーブルで、複合主キーを持つ（基本設計 03 §2）。
-- ビジネスルールのうち制約で表現できるものは制約で表現する。
-- アプリケーション側のチェックだけに頼ると、バグや別経路からの書き込みで破られる。

CREATE TABLE likes (
    user_id    BIGINT      NOT NULL,
    post_id    BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- 同じ投稿への二重いいねを主キーで防ぐ
    CONSTRAINT likes_pkey PRIMARY KEY (user_id, post_id),

    CONSTRAINT likes_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT likes_post_id_fkey
        FOREIGN KEY (post_id) REFERENCES posts (id) ON DELETE CASCADE
);

COMMENT ON TABLE likes IS 'いいね。件数は posts.like_count に非正規化して保持する';

-- 基本設計 03 §3 インデックス #11: 投稿へのいいね一覧
-- 主キーは (user_id, post_id) の順なので、post_id 単独の検索には使えない。
CREATE INDEX likes_post_id_idx ON likes (post_id);

CREATE TABLE follows (
    follower_id BIGINT      NOT NULL,
    followee_id BIGINT      NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT follows_pkey PRIMARY KEY (follower_id, followee_id),

    CONSTRAINT follows_follower_id_fkey
        FOREIGN KEY (follower_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT follows_followee_id_fkey
        FOREIGN KEY (followee_id) REFERENCES users (id) ON DELETE CASCADE,

    -- BR-05: 自分自身をフォローできない
    CONSTRAINT follows_not_self_check
        CHECK (follower_id <> followee_id)
);

COMMENT ON TABLE follows IS 'フォロー関係。ブロック時は双方向に解除される（BR-08）';

-- 基本設計 03 §3 インデックス #9: フォロワー一覧
CREATE INDEX follows_followee_id_idx ON follows (followee_id);

CREATE TABLE blocks (
    blocker_id BIGINT      NOT NULL,
    blocked_id BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT blocks_pkey PRIMARY KEY (blocker_id, blocked_id),

    CONSTRAINT blocks_blocker_id_fkey
        FOREIGN KEY (blocker_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT blocks_blocked_id_fkey
        FOREIGN KEY (blocked_id) REFERENCES users (id) ON DELETE CASCADE,

    -- BR-06: 自分自身をブロックできない
    CONSTRAINT blocks_not_self_check
        CHECK (blocker_id <> blocked_id)
);

COMMENT ON TABLE blocks IS 'ブロック。ブロックされた側には知らされない（BR-10）';

-- 基本設計 03 §3 インデックス #8（follows PK）・#10（likes PK）・#12（blocks PK）は
-- 主キーが自動で作るため、CREATE INDEX は書かない。
