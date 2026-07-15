-- Support multi-use / unlimited invite codes.
-- max_uses: NULL = unlimited, otherwise the maximum number of redemptions.

ALTER TABLE invite_codes ADD COLUMN max_uses  INTEGER;
ALTER TABLE invite_codes ADD COLUMN use_count INTEGER NOT NULL DEFAULT 0;

-- Existing codes were single-use.
UPDATE invite_codes SET max_uses = 1;
UPDATE invite_codes SET use_count = 1 WHERE used_by IS NOT NULL;
