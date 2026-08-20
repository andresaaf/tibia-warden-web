-- Areas group the Echo Warden creatures found at one in-game location. A
-- creature can belong to several areas (many-to-many), mirroring the composite
-- key of warden_kills. The tables are group-agnostic so a future per-group
-- "area checked in the last 30 min" respawn tracker can layer on without change.

CREATE TABLE areas (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT      NOT NULL UNIQUE,
    sort_order INTEGER   NOT NULL DEFAULT 0
);

CREATE TABLE area_creatures (
    area_id     BIGINT NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    creature_id BIGINT NOT NULL REFERENCES creatures(id) ON DELETE CASCADE,
    PRIMARY KEY (area_id, creature_id)
);
CREATE INDEX idx_area_creatures_creature ON area_creatures(creature_id);
