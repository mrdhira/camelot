CREATE TABLE card_docs (
    card_id TEXT NOT NULL REFERENCES cards (id),
    doc_id TEXT NOT NULL,
    PRIMARY KEY (card_id, doc_id)
);
CREATE INDEX card_docs_doc ON card_docs (doc_id);
