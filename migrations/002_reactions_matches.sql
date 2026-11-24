CREATE TABLE reactions (
    actor_id text NOT NULL REFERENCES users(id),
    target_id text NOT NULL REFERENCES users(id),
    PRIMARY KEY (actor_id, target_id),
    CONSTRAINT reactions_distinct_users CHECK (actor_id <> target_id)
);

CREATE TABLE matches (
    low_id text NOT NULL,
    high_id text NOT NULL,
    active boolean NOT NULL,
    PRIMARY KEY (low_id, high_id),
    FOREIGN KEY (low_id, high_id) REFERENCES pairs(low_id, high_id)
);
