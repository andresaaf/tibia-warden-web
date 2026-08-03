-- Charm Points are a difficulty-weighted score. The weights used to live only in
-- the Highscores CASE; this lookup table makes them the single source of truth so
-- the roster stats and future pay-per-charm pricing can reuse one definition.

CREATE TABLE charm_weights (
    difficulty TEXT    PRIMARY KEY,
    points     INTEGER NOT NULL
);

INSERT INTO charm_weights (difficulty, points) VALUES
    ('Harmless', 1),
    ('Trivial', 2),
    ('Easy', 5),
    ('Medium', 10),
    ('Hard', 15),
    ('Challenging', 30);
