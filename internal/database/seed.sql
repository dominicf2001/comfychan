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
    FOREIGN KEY (board_slug) REFERENCES boards(slug) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    thread_id INTEGER NOT NULL,
    number INTEGER NOT NULL ,
    author TEXT DEFAULT 'Anonymous',
    body TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    media_path TEXT NOT NULL DEFAULT '',
    thumb_path TEXT NOT NULL DEFAULT '',
    ip_hash TEXT NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);

-- ======================
-- Seed data
-- ======================

-- Boards

INSERT INTO boards (slug, name, tag) VALUES 
    ('comfy', 'Comfy', 'Be comfy, fren'),
    ('g', 'Technology', 'Beep boop');
