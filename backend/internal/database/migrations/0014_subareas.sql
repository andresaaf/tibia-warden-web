-- Introduce a subarea layer between areas and creatures. Each subarea is one
-- echo-raid spawn point (the unit the future 30-min respawn checker needs); an
-- area groups its subareas. Creatures attach to subareas only; the warden-list
-- area view derives its creature set as the DISTINCT union across an area's
-- subareas. area_creatures is replaced by subarea_creatures. Area data is
-- seed-derived (no user rows reference it), so it is safe to rebuild on the next
-- seed.

CREATE TABLE subareas (
    id         BIGSERIAL PRIMARY KEY,
    area_id    BIGINT  NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    UNIQUE (area_id, name)
);

CREATE TABLE subarea_creatures (
    subarea_id  BIGINT NOT NULL REFERENCES subareas(id) ON DELETE CASCADE,
    creature_id BIGINT NOT NULL REFERENCES creatures(id) ON DELETE CASCADE,
    PRIMARY KEY (subarea_id, creature_id)
);
CREATE INDEX idx_subarea_creatures_creature ON subarea_creatures(creature_id);

DROP TABLE area_creatures;
