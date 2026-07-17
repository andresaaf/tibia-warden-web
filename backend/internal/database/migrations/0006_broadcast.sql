-- Link announcements created by a single multi-group broadcast so a kill can
-- cascade to its siblings. NULL for single-group posts.

ALTER TABLE announcements ADD COLUMN broadcast_id TEXT;
CREATE INDEX idx_announcements_broadcast_id
    ON announcements(broadcast_id) WHERE broadcast_id IS NOT NULL;
