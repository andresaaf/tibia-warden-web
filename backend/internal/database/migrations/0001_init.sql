-- Initial schema for the Tibia Echo Warden community app.

CREATE TABLE users (
    id              BIGSERIAL   PRIMARY KEY,
    discord_id      TEXT        NOT NULL UNIQUE,
    discord_username TEXT       NOT NULL,
    discord_avatar  TEXT        NOT NULL DEFAULT '',
    character_name  TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token      TEXT        PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);

CREATE TABLE creatures (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT      NOT NULL UNIQUE,
    difficulty TEXT      NOT NULL,
    image_url  TEXT      NOT NULL DEFAULT ''
);
CREATE INDEX idx_creatures_difficulty ON creatures(difficulty);

CREATE TABLE warden_kills (
    user_id     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    creature_id BIGINT      NOT NULL REFERENCES creatures(id) ON DELETE CASCADE,
    killed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, creature_id)
);

CREATE TABLE groups (
    id          BIGSERIAL   PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    visibility  TEXT        NOT NULL DEFAULT 'public',
    owner_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_groups_visibility ON groups(visibility);

CREATE TABLE group_members (
    group_id  BIGINT      NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id   BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      TEXT        NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX idx_group_members_user_id ON group_members(user_id);

CREATE TABLE invite_codes (
    id         BIGSERIAL   PRIMARY KEY,
    group_id   BIGINT      NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    code       TEXT        NOT NULL UNIQUE,
    created_by BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    used_by    BIGINT      REFERENCES users(id) ON DELETE SET NULL,
    used_at    TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_invite_codes_group_id ON invite_codes(group_id);

CREATE TABLE announcements (
    id          BIGSERIAL   PRIMARY KEY,
    group_id    BIGINT      NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    creature_id BIGINT      NOT NULL REFERENCES creatures(id) ON DELETE RESTRICT,
    author_id   BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    location    TEXT        NOT NULL DEFAULT '',
    note        TEXT        NOT NULL DEFAULT '',
    gold_cost   INTEGER     NOT NULL DEFAULT 0,
    status      TEXT        NOT NULL DEFAULT 'open',
    killed_at   TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_announcements_group_id ON announcements(group_id);

CREATE TABLE announcement_responses (
    announcement_id BIGINT      NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    user_id         BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          TEXT        NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (announcement_id, user_id)
);

CREATE TABLE announcement_claims (
    announcement_id BIGINT      NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    user_id         BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    claimed_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (announcement_id, user_id)
);
