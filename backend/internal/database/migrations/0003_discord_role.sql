-- Optional Discord role to @mention when an announcement is posted.

ALTER TABLE groups ADD COLUMN discord_role_id   TEXT NOT NULL DEFAULT '';
ALTER TABLE groups ADD COLUMN discord_role_name TEXT NOT NULL DEFAULT '';
