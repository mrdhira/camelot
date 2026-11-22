CREATE TABLE doc_sync_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    baselined_at TIMESTAMP NOT NULL
);
INSERT INTO doc_sync_state (id, baselined_at) SELECT 1, MIN(synced_at) FROM doc_snapshots HAVING COUNT(*) > 0;
