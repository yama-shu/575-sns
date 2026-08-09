-- reports
--
-- 基本設計 03 §2 のテーブル定義に対応する。
-- 状態遷移は基本設計 02 §4（未対応 → 対応済み / 却下）。

CREATE TABLE reports (
    id          BIGSERIAL    PRIMARY KEY,
    reporter_id BIGINT       NOT NULL,
    post_id     BIGINT       NOT NULL,
    reason      VARCHAR(20)  NOT NULL,
    comment     VARCHAR(500),
    status      VARCHAR(10)  NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    -- 対応した運営
    resolved_by BIGINT,

    CONSTRAINT reports_reporter_id_fkey
        FOREIGN KEY (reporter_id) REFERENCES users (id),
    CONSTRAINT reports_post_id_fkey
        FOREIGN KEY (post_id) REFERENCES posts (id),
    CONSTRAINT reports_resolved_by_fkey
        FOREIGN KEY (resolved_by) REFERENCES users (id),

    -- 同一利用者による同一投稿への重複通報を防ぐ（基本設計 02 §4）。
    -- 連続通報による嫌がらせを防ぐため、1人1投稿につき1件まで。
    CONSTRAINT reports_reporter_id_post_id_key UNIQUE (reporter_id, post_id),

    CONSTRAINT reports_reason_check
        CHECK (reason IN ('spam', 'harassment', 'inappropriate', 'other')),

    CONSTRAINT reports_status_check
        CHECK (status IN ('pending', 'resolved', 'rejected')),

    -- 未対応なら対応日時と対応者が無く、処理済みなら両方ある。
    -- 「対応済みなのに誰がいつ対応したか分からない」状態を作れなくする。
    CONSTRAINT reports_resolution_consistency_check
        CHECK (
            (status = 'pending'  AND resolved_at IS NULL     AND resolved_by IS NULL)
         OR (status <> 'pending' AND resolved_at IS NOT NULL AND resolved_by IS NOT NULL)
        )
);

COMMENT ON TABLE  reports        IS '通報。投稿単位で運営が対応する';
COMMENT ON COLUMN reports.reason IS 'spam / harassment / inappropriate / other';
COMMENT ON COLUMN reports.status IS 'pending / resolved / rejected';

-- 基本設計 03 §3 インデックス #14: 運営の通報一覧
--
-- 部分インデックスにする理由（§3）: 対応済みの通報は一覧に出ない。
-- 運用が続くほど resolved が積み上がるが、部分インデックスなら影響を受けない。
CREATE INDEX reports_pending_idx
    ON reports (created_at)
    WHERE status = 'pending';

-- BR-07（自分自身の投稿を通報できない）は reporter_id と posts.author_id の
-- 突き合わせが必要で、単一行の CHECK では表現できない。
-- アプリケーション側で担保する。
--
-- インデックス #13 は上記の UNIQUE 制約が自動で作るため、CREATE INDEX は書かない。
