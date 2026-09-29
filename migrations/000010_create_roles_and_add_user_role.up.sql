CREATE TABLE IF NOT EXISTS roles (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    level INT NOT NULL,
    description TEXT
);

INSERT INTO roles (name, level, description)
VALUES 
    ('user', 1, 'Default user role with standard permissions'),
    ('moderator', 2, 'Moderator role with content moderation permissions'),
    ('admin', 3, 'Administrator role with full system access')
ON CONFLICT (name) DO NOTHING;

ALTER TABLE users ADD COLUMN IF NOT EXISTS role_id BIGINT REFERENCES roles(id);

UPDATE users
SET role_id = (SELECT id FROM roles WHERE name = 'user')
WHERE role_id IS NULL;

ALTER TABLE users ALTER COLUMN role_id SET NOT NULL;
