-- Per-group access mode. 'free' (default) or 'pay_to_attend', where attendees
-- pay per Warden, in-game after the kill (not handled here). The price is a
-- fixed gold amount per creature difficulty (attend_prices: {"Hard": 30000, ...};
-- missing = free), multiplied by uncommon_multiplier for Uncommon creatures.
-- Prices are kept when switching back to free so they return on re-enable.
ALTER TABLE groups ADD COLUMN access_mode TEXT NOT NULL DEFAULT 'free'
    CHECK (access_mode IN ('free', 'pay_to_attend'));
ALTER TABLE groups ADD COLUMN attend_prices JSONB NOT NULL DEFAULT '{}';
ALTER TABLE groups ADD COLUMN uncommon_multiplier NUMERIC(6, 2) NOT NULL DEFAULT 1
    CHECK (uncommon_multiplier > 0);

-- Snapshot of the computed price when the announcement was posted, so later
-- price changes only affect new announcements. 0 = free.
ALTER TABLE announcements ADD COLUMN attend_price BIGINT NOT NULL DEFAULT 0;

-- gold_cost was an early placeholder for this price; it never had an input
-- (always 0) and is superseded by attend_price.
ALTER TABLE announcements DROP COLUMN gold_cost;
