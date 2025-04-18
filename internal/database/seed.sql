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
    author TEXT DEFAULT 'Anonymous',
    body TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);

-- ======================
-- Seed data
-- ======================

-- Boards

INSERT INTO boards (slug, name, tag) VALUES 
    ('comfy', 'Comfy', 'Be comfy, fren'),
    ('g', 'Technology', 'Beep boop');

-- Welcome threads

  -- /comfy/
    INSERT INTO threads (board_slug, subject) VALUES ( 
        'comfy',
        'Welcome to /comfy/.' 
    );
    INSERT INTO posts (thread_id, body) VALUES (
        (SELECT id FROM threads WHERE subject LIKE 'Welcome to /comfy/.%'),
        'Post comfy. Be nice.'
    );
    INSERT INTO posts (thread_id, body) VALUES (
        (SELECT id FROM threads WHERE subject LIKE 'Welcome to /comfy/.%'),
        'Henlo fren.'
    );
