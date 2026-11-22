CREATE TABLE feed_seen (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    seen_at TIMESTAMP NOT NULL
);
