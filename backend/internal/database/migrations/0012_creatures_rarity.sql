-- Track each creature's rarity (Common/Uncommon), derived from the TibiaWiki
-- occurrence. Existing rows default to '' and backfill on the next sync.

ALTER TABLE creatures ADD COLUMN rarity TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_creatures_rarity ON creatures(rarity);
