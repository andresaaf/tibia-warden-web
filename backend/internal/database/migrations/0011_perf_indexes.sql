-- Support the group roster metrics and highscores aggregation, which filter
-- announcements by author / status / kill+create time (previously only group_id
-- was indexed).

-- Members: authored + score aggregation grouped by author within a group.
CREATE INDEX idx_announcements_group_author
    ON announcements(group_id, author_id);

-- Members: attended/score scan of killed announcements bounded by kill window.
CREATE INDEX idx_announcements_group_killed
    ON announcements(group_id, killed_at) WHERE status = 'killed';

-- Highscores: per-author aggregation over all killed announcements.
CREATE INDEX idx_announcements_killed_author
    ON announcements(author_id) WHERE status = 'killed';
