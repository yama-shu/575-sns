-- posts
--
-- 基本設計 03 §2 のテーブル定義に対応する。
-- 判定結果は投稿の一部であり、投稿時に確定して以後変更されない（基本設計 02 BR-04）。

CREATE TABLE posts (
    -- カーソルページネーションのカーソルを兼ねる（基本設計 03 §5）
    id          BIGSERIAL    PRIMARY KEY,
    author_id   BIGINT       NOT NULL,
    -- 上限100文字の根拠は基本設計 03 §2。API 側でも同じ上限を検証する（NFR-04-06）。
    body        VARCHAR(100) NOT NULL,
    -- prosody が推定したカタカナ読み。判定の根拠として保存する。
    reading     VARCHAR(100) NOT NULL,
    -- hacho（破調）は保存されない。投稿時点で弾く（BR-01）。
    verdict     VARCHAR(10)  NOT NULL,
    -- 区切りは文字位置で持つ。本文を3つに分割して保存しないため、不整合が構造的に起きない。
    break1      SMALLINT     NOT NULL,
    break2      SMALLINT     NOT NULL,
    mora_kami   SMALLINT     NOT NULL,
    mora_naka   SMALLINT     NOT NULL,
    mora_shimo  SMALLINT     NOT NULL,
    visibility  VARCHAR(10)  NOT NULL DEFAULT 'public',
    status      VARCHAR(10)  NOT NULL DEFAULT 'published',
    -- 非正規化（基本設計 03 §4）。likes からの集計を毎回走らせないため。
    like_count  INTEGER      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,

    CONSTRAINT posts_author_id_fkey
        FOREIGN KEY (author_id) REFERENCES users (id),

    CONSTRAINT posts_verdict_check
        CHECK (verdict IN ('teikei', 'kyoyo')),

    CONSTRAINT posts_visibility_check
        CHECK (visibility IN ('public', 'followers')),

    CONSTRAINT posts_status_check
        CHECK (status IN ('published', 'hidden', 'deleted')),

    -- ADR-0001 の許容範囲を DB の制約としても表現する。
    -- 判定ロジックの誤りがサイレントにデータを汚染するのを、最後の砦として防ぐ。
    -- 「ズレは最大1句」という条件は CHECK で表しにくいため、アプリケーション側で担保する。
    CONSTRAINT posts_mora_kami_check  CHECK (mora_kami  BETWEEN 4 AND 6),
    CONSTRAINT posts_mora_naka_check  CHECK (mora_naka  BETWEEN 6 AND 8),
    CONSTRAINT posts_mora_shimo_check CHECK (mora_shimo BETWEEN 4 AND 6),

    -- 区切りは本文の内側で、かつ break1 < break2 でなければならない。
    -- body[0:break1] / body[break1:break2] / body[break2:] で復元するため、
    -- この順序が崩れると本文を3句に分けられなくなる。
    CONSTRAINT posts_break_order_check
        CHECK (0 < break1 AND break1 < break2 AND break2 < char_length(body)),

    -- 異常な減算を検出する（基本設計 03 §4）
    CONSTRAINT posts_like_count_check
        CHECK (like_count >= 0),

    -- 削除済みなら削除日時があり、そうでなければ無い。
    CONSTRAINT posts_deleted_at_consistency_check
        CHECK ((status = 'deleted') = (deleted_at IS NOT NULL))
);

COMMENT ON TABLE  posts            IS '投稿（詠まれた五七五）';
COMMENT ON COLUMN posts.reading    IS 'prosody が推定したカタカナ読み。判定の根拠';
COMMENT ON COLUMN posts.verdict    IS 'teikei（定型）/ kyoyo（許容）。hacho は保存されない';
COMMENT ON COLUMN posts.break1     IS '上五と中七の境界。body の文字位置';
COMMENT ON COLUMN posts.break2     IS '中七と下五の境界。body の文字位置';
COMMENT ON COLUMN posts.like_count IS '非正規化。likes と同一トランザクションでアトミックに更新する';

-- 基本設計 03 §3 インデックス #6: 全体タイムライン
--
-- 部分インデックスにする理由（§3）: 論理削除でレコードが蓄積し続けるため、
-- 通常のインデックスだと削除済み投稿が増えるほど肥大して検索が遅くなる。
-- published かつ public の投稿だけを載せれば、削除済みが増えても太らない。
CREATE INDEX posts_public_timeline_idx
    ON posts (id DESC)
    WHERE status = 'published' AND visibility = 'public';

-- 基本設計 03 §3 インデックス #7: フォロー中タイムライン、ユーザーページ
--
-- visibility を条件に含めない。フォロー中タイムラインには followers 限定の投稿も出るため。
CREATE INDEX posts_author_timeline_idx
    ON posts (author_id, id DESC)
    WHERE status = 'published';
