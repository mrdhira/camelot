CREATE TABLE card_deps (
    card_id TEXT NOT NULL REFERENCES cards (id),
    depends_on_id TEXT NOT NULL REFERENCES cards (id),
    PRIMARY KEY (card_id, depends_on_id),
    CHECK (card_id <> depends_on_id)
);
CREATE INDEX card_deps_depends_on ON card_deps (depends_on_id);
