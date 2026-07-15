-- Discord bot integration: link groups to a Discord channel and mirror announcements.

ALTER TABLE groups ADD COLUMN discord_guild_id   TEXT NOT NULL DEFAULT '';
ALTER TABLE groups ADD COLUMN discord_channel_id TEXT NOT NULL DEFAULT '';

ALTER TABLE announcements ADD COLUMN discord_message_id TEXT NOT NULL DEFAULT '';

CREATE TABLE discord_link_codes (
    code       TEXT        PRIMARY KEY,
    group_id   BIGINT      NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_by BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_discord_link_codes_group_id ON discord_link_codes(group_id);
