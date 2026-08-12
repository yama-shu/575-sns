-- 運営を示す列（FR-05-03 / FR-05-04）。
--
-- **ロール表を作らない。** 区分は「運営かどうか」の1つしかなく、
-- 表にしても入る行が増えない。必要になった時点で作り直す。
--
-- **この列を真にする API は作らない。** 権限を与える経路は、壊れたときの
-- 被害が最も大きい。運用者がひとりである以上、手作業で足りる（#74）。
--
--     UPDATE users SET is_admin = true WHERE handle = '...';
ALTER TABLE users ADD COLUMN is_admin boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN users.is_admin IS '運営かどうか。付与は DB を直接更新する';
