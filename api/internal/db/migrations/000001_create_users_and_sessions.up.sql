-- users / sessions
--
-- 基本設計 03 §2 のテーブル定義に対応する。
-- 利用者と、その認証状態（セッション）を持つ。

CREATE TABLE users (
    id            BIGSERIAL    PRIMARY KEY,
    -- 識別名 @xxx。退会後も再利用させないため、退会しても行を残す（基本設計 02）。
    handle        VARCHAR(20)  NOT NULL,
    email         VARCHAR(255) NOT NULL,
    -- ソルト付きハッシュ（NFR-04-01）。平文は保存しない。
    password_hash TEXT         NOT NULL,
    display_name  VARCHAR(50)  NOT NULL,
    -- 自己紹介。五七五である必要はない（FR-01-03）。
    bio           VARCHAR(200),
    avatar_url    TEXT,
    status        VARCHAR(10)  NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,

    CONSTRAINT users_handle_key UNIQUE (handle),
    CONSTRAINT users_email_key  UNIQUE (email),

    -- 基本設計 02 §3 の状態遷移。ここに無い値は入らない。
    CONSTRAINT users_status_check
        CHECK (status IN ('active', 'suspended', 'deleted')),

    -- 識別名は半角英数字とアンダースコアのみ（基本設計 03 §2）
    CONSTRAINT users_handle_format_check
        CHECK (handle ~ '^[A-Za-z0-9_]+$'),

    -- 退会済みなら退会日時があり、退会済みでないなら無い。
    -- 状態と日時が食い違ったデータを構造的に作れなくする。
    CONSTRAINT users_deleted_at_consistency_check
        CHECK ((status = 'deleted') = (deleted_at IS NOT NULL))
);

COMMENT ON TABLE  users        IS '利用者';
COMMENT ON COLUMN users.handle IS '識別名 @xxx。退会後も再利用させない';
COMMENT ON COLUMN users.status IS 'active / suspended / deleted';

CREATE TABLE sessions (
    -- 32バイトの乱数を base64url した43文字。連番・推測可能な値を使わない（ADR-0006）。
    id               CHAR(43)    PRIMARY KEY,
    user_id          BIGINT      NOT NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- スライディング期限の起点（ADR-0006）
    last_accessed_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- 利用停止・退会時にセッションを即座に破棄する（基本設計 02 §3）。
    -- 破棄漏れでログイン状態が残るのを防ぐため、DB 側で連鎖削除する。
    CONSTRAINT sessions_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

COMMENT ON TABLE  sessions    IS 'ログインセッション（ADR-0006: サーバー側セッション）';
COMMENT ON COLUMN sessions.id IS '32バイトの乱数を base64url した文字列';

-- 基本設計 03 §3 インデックス #4: 利用停止時の一括削除
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

-- 基本設計 03 §3 インデックス #5: 期限切れセッションの定期削除
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- インデックス #1 users(handle) / #2 users(email) / #3 sessions(id) は
-- UNIQUE 制約・主キーが自動で作るため、CREATE INDEX は書かない。
-- 「必要な場所にだけ張る」（基本設計 03 §3）ため、重複したインデックスは作らない。
