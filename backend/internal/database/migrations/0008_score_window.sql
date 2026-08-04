-- Per-group setting controlling how far back the roster "Score" metric counts.
-- score_window: 'forever' (default), 'month' (since the 1st of the current
-- month), or 'week' (since the start of the current week, Monday).

ALTER TABLE groups ADD COLUMN score_window TEXT NOT NULL DEFAULT 'forever';
