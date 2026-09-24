-- Price per difficulty AND rarity instead of a single uncommon multiplier.
-- attend_prices goes from {difficulty: gold} to {rarity: {difficulty: gold}};
-- existing prices become the Common column, the multiplier folded into Uncommon.
UPDATE groups
SET attend_prices = jsonb_build_object(
        'Common', attend_prices,
        'Uncommon', (
            SELECT COALESCE(jsonb_object_agg(e.key, ROUND(e.value::numeric * uncommon_multiplier)::bigint), '{}'::jsonb)
            FROM jsonb_each_text(attend_prices) AS e(key, value)
        ))
WHERE attend_prices <> '{}'::jsonb;

ALTER TABLE groups DROP COLUMN uncommon_multiplier;
