CREATE TABLE blocks (
    blocker_id text NOT NULL REFERENCES users(id),
    blocked_id text NOT NULL REFERENCES users(id),
    PRIMARY KEY (blocker_id, blocked_id),
    CONSTRAINT blocks_distinct_users CHECK (blocker_id <> blocked_id)
);
