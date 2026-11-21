CREATE TABLE cards (
    id TEXT PRIMARY KEY NOT NULL,
    milestone_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('milestone', 'feature', 'task', 'bug', 'research', 'tech-debt', 'feature-debt', 'product-debt', 'security-debt')),
    status TEXT NOT NULL CHECK (status IN ('backlog', 'todo', 'plan', 'wip', 'qa', 'review', 'done')),
    assignee TEXT NOT NULL DEFAULT '',
    waiting TEXT NOT NULL DEFAULT '',
    folder TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);
CREATE INDEX cards_status ON cards (status) WHERE deleted_at IS NULL;
