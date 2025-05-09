PRAGMA foreign_keys = ON;

-- ======================
-- Tables
-- ======================

CREATE TABLE IF NOT EXISTS boards (
    id INTEGER PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    tag TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS threads ( 
    id INTEGER PRIMARY KEY AUTOINCREMENT, 
    board_slug TEXT NOT NULL,
    subject TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    bumped_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    pinned BOOLEAN NOT NULL DEFAULT 0,
    locked BOOLEAN NOT NULL DEFAULT 0,
    FOREIGN KEY (board_slug) REFERENCES boards(slug) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    thread_id INTEGER NOT NULL,
    number INTEGER NOT NULL ,
    banned BOOLEAN NOT NULL DEFAULT 0,
    author TEXT DEFAULT 'Anonymous',
    body TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    media_path TEXT NOT NULL DEFAULT '',
    thumb_path TEXT NOT NULL DEFAULT '',
    ip_hash TEXT NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS admins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    password TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS bans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ip_hash TEXT NOT NULL UNIQUE,
    reason TEXT NOT NULL,
    expiration DATETIME NOT NULL 
);

-- ======================
-- Seed data
-- ======================

-- Boards

INSERT INTO boards (slug, name, tag) VALUES 
    ('c', 'Comfy', 'Be comfy, fren'),
    ('r', 'Robots', 'Beep, boop'),
    ('gn', 'Goon', 'God is watching');

INSERT INTO admins (username, password) VALUES
    ('admin', '$2a$10$vRP4/9O6SwyUziEUtBLQM.r9C2WujIIZ6yEgqGjhlBaFPvtpfdHPC');
