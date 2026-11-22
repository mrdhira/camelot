CREATE TABLE doc_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    doc_id TEXT NOT NULL,
    actor TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('created', 'edited', 'status', 'removed')),
    from_value TEXT NOT NULL DEFAULT '',
    to_value TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL
);
CREATE INDEX doc_events_created ON doc_events (created_at);
CREATE INDEX doc_events_doc ON doc_events (doc_id, id);
