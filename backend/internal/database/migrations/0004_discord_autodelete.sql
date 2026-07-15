-- Per-group setting to auto-delete the mirrored Discord message after a kill.
-- discord_autodelete_seconds: -1 = Never (default), 0 = immediately on kill,
-- otherwise the number of seconds after the kill to delete the message.

ALTER TABLE groups ADD COLUMN discord_autodelete_seconds INTEGER NOT NULL DEFAULT -1;

-- When set, the sweeper deletes the announcement's Discord message at/after this time.
ALTER TABLE announcements ADD COLUMN discord_delete_at TIMESTAMPTZ;
CREATE INDEX idx_announcements_discord_delete_at
    ON announcements(discord_delete_at) WHERE discord_delete_at IS NOT NULL;
