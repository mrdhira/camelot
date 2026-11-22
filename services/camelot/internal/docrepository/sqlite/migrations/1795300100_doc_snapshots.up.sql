CREATE TABLE doc_snapshots (
    doc_id TEXT PRIMARY KEY NOT NULL,
    path TEXT NOT NULL,
    title TEXT NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    content_sha256 TEXT NOT NULL,
    modified_at TIMESTAMP NOT NULL,
    synced_at TIMESTAMP NOT NULL
);
