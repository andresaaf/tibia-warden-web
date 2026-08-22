-- Optional in-game map coordinate for an announcement. All three columns are
-- NULL together when the announcer never marked a spot (the default), so old
-- rows and map-less posts are unchanged. When set, (map_x, map_y) are absolute
-- Tibia world coordinates and map_z is the floor (7 = ground, 0 highest,
-- 15 deepest). Used to render an interactive map on the site and a static map
-- image in the Discord embed.

ALTER TABLE announcements
    ADD COLUMN map_x INTEGER,
    ADD COLUMN map_y INTEGER,
    ADD COLUMN map_z INTEGER;
