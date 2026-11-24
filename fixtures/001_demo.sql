INSERT INTO users (id, name, age, city, interests, active) VALUES
    ('u1', 'Анна', 25, 'Москва', ARRAY['music', 'travel'], true),
    ('u2', 'Борис', 28, 'Москва', ARRAY['music', 'sport'], true),
    ('u3', 'Вера', 31, 'Казань', ARRAY['travel', 'books'], true),
    ('u4', 'Глеб', 22, 'Москва', ARRAY['music'], true),
    ('u5', 'Дина', 27, 'Москва', ARRAY['music'], false),
    ('u6', 'Егор', 35, 'Москва', ARRAY['travel'], true),
    ('u7', 'Игорь', 29, 'Москва', ARRAY['travel', 'go'], true),
    ('u8', 'Ольга', 26, 'Казань', ARRAY['travel', 'books'], true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO pairs (low_id, high_id) VALUES
    ('u1', 'u2'),
    ('u1', 'u4'),
    ('u1', 'u6')
ON CONFLICT DO NOTHING;
