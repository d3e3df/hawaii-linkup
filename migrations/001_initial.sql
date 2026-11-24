CREATE TABLE users (
    id text PRIMARY KEY,
    name text NOT NULL CHECK (length(btrim(name)) > 0),
    age integer NOT NULL CHECK (age BETWEEN 18 AND 120),
    city text NOT NULL CHECK (length(btrim(city)) > 0),
    interests text[] NOT NULL,
    active boolean NOT NULL,
    CONSTRAINT users_id_format CHECK (id ~ '^u[1-9][0-9]*$')
);

CREATE TABLE pairs (
    low_id text NOT NULL REFERENCES users(id),
    high_id text NOT NULL REFERENCES users(id),
    PRIMARY KEY (low_id, high_id),
    CONSTRAINT pairs_canonical_order CHECK (low_id < high_id)
);
