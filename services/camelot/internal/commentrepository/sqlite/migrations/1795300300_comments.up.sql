CREATE TABLE comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL CHECK (kind IN ('card', 'doc')),
    target_id TEXT NOT NULL,
    section TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL,
    actor TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    resolves_id INTEGER REFERENCES comments (id),
    UNIQUE (resolves_id)
);
CREATE INDEX comments_target ON comments (kind, target_id);
