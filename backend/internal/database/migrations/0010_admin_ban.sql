-- Site-wide moderation: a global admin flag and a per-account ban flag.
-- is_admin gates the admin area (bootstrap the first admin manually via SQL).
-- banned locks an account out of the website entirely (checked in requireAuth).

ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN banned   BOOLEAN NOT NULL DEFAULT false;
