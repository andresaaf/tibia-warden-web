-- Respawn checker: a user "checks" a subarea when they step on it; a wild echo
-- can respawn there 20-30 minutes later. One row per (subarea, user), upserted
-- on every re-check, so the table stays bounded. prev_checked_at remembers the
-- check being replaced so a mis-click can be undone. Rows reference subareas by
-- id, so the seeder now upserts areas in place (instead of clearing them) to
-- keep ids stable.
CREATE TABLE subarea_checks (
    subarea_id      BIGINT      NOT NULL REFERENCES subareas(id) ON DELETE CASCADE,
    user_id         BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    prev_checked_at TIMESTAMPTZ,
    PRIMARY KEY (subarea_id, user_id)
);
CREATE INDEX idx_subarea_checks_checked_at ON subarea_checks(checked_at);

-- Subareas a user pinned to the top of their checker.
CREATE TABLE subarea_favorites (
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subarea_id BIGINT      NOT NULL REFERENCES subareas(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, subarea_id)
);

-- Whether a member's checks are visible to (and pushed live to) this group.
ALTER TABLE group_members ADD COLUMN share_checks BOOLEAN NOT NULL DEFAULT true;
