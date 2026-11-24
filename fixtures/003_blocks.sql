INSERT INTO blocks (blocker_id, blocked_id) VALUES
    ('u1', 'u4')
ON CONFLICT DO NOTHING;

UPDATE matches SET active = false
WHERE (low_id, high_id) IN (
    SELECT p.low_id, p.high_id
    FROM pairs p
    JOIN blocks b ON (
        (b.blocker_id = p.low_id AND b.blocked_id = p.high_id)
        OR (b.blocker_id = p.high_id AND b.blocked_id = p.low_id)
    )
);
