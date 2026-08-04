-- The per-group score_window setting (migration 0008) is superseded by the
-- viewer-controlled roster period switch (Lifetime / Previous Month / Current
-- Month), which windows every roster metric rather than just Score.

ALTER TABLE groups DROP COLUMN score_window;
