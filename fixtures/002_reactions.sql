INSERT INTO pairs (low_id, high_id) VALUES
    ('u1', 'u3'),
    ('u7', 'u8')
ON CONFLICT DO NOTHING;

INSERT INTO reactions (actor_id, target_id) VALUES
    ('u1', 'u6'),
    ('u2', 'u1'),
    ('u7', 'u8'),
    ('u8', 'u7')
ON CONFLICT DO NOTHING;

INSERT INTO matches (low_id, high_id, active) VALUES
    ('u7', 'u8', true)
ON CONFLICT DO NOTHING;
