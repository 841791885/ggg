ALTER TABLE users
    ADD COLUMN role ENUM('admin', 'customer') NOT NULL DEFAULT 'customer' AFTER password_hash,
    ADD INDEX idx_users_role (role);

UPDATE users SET role = 'admin' WHERE username = 'admin';
