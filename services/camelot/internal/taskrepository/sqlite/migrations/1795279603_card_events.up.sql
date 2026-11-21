CREATE TABLE card_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    card_id TEXT NOT NULL REFERENCES cards (id),
    actor TEXT NOT NULL,
    field TEXT NOT NULL,
    from_value TEXT NOT NULL DEFAULT '',
    to_value TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL
);
CREATE INDEX card_events_card ON card_events (card_id, id);
CREATE INDEX card_events_created ON card_events (created_at);
