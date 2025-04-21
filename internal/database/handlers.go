package database

import (
	"database/sql"
)

type Queryer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

func GetBoards(db *sql.DB) ([]Board, error) {
	rows, err := db.Query(`
		SELECT id, name, slug, tag 
		FROM boards ORDER BY slug`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Board
	for rows.Next() {
		var b Board
		err := rows.Scan(&b.Id, &b.Name, &b.Slug, &b.Tag)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}

	return result, rows.Err()
}

func GetBoard(db *sql.DB, slug string) (Board, error) {
	row := db.QueryRow(`
		SELECT id, name, slug, tag 
		FROM boards 
		WHERE slug = ?`, slug)

	var result Board
	err := row.Scan(&result.Id, &result.Name, &result.Slug, &result.Tag)
	if err != nil {
		return Board{}, err
	}

	return result, row.Err()
}

func GetThreads(db *sql.DB, boardSlug string) ([]Thread, error) {
	rows, err := db.Query(`
		SELECT id, board_slug, subject, created_at, bumped_at 
		FROM threads 
		WHERE board_slug = ?`, boardSlug)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Thread
	for rows.Next() {
		var t Thread
		err := rows.Scan(&t.Id, &t.BoardSlug, &t.Subject, &t.CreatedAt, &t.BumpedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}

	return result, rows.Err()
}

func GetThread(db *sql.DB, threadId int) (Thread, error) {
	row := db.QueryRow(`
		SELECT id, board_slug, subject, created_at, bumped_at 
		FROM threads 
		WHERE id = ?`, threadId)

	var thread Thread
	err := row.Scan(&thread.Id, &thread.BoardSlug, &thread.Subject, &thread.CreatedAt, &thread.BumpedAt)
	if err != nil {
		return Thread{}, err
	}

	return thread, row.Err()
}

func GetPosts(db *sql.DB, threadId int) ([]Post, error) {
	rows, err := db.Query(`
		SELECT id, thread_id, author, body, created_at, media_path, ip_hash, number 
		FROM posts 
		WHERE thread_id = ?`, threadId)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Post
	for rows.Next() {
		var p Post
		err := rows.Scan(&p.Id, &p.ThreadId, &p.Author, &p.Body, &p.CreatedAt, &p.MediaPath, &p.IpHash, &p.Number)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}

	return result, rows.Err()
}

func GetOriginalPost(db *sql.DB, threadId int) (Post, error) {
	row := db.QueryRow(`
		SELECT id, thread_id, author, body, created_at, media_path, ip_hash, number
		FROM posts 
		WHERE thread_id = ? 
		ORDER BY created_at ASC LIMIT 1`, threadId)

	var r Post
	err := row.Scan(&r.Id, &r.ThreadId, &r.Author, &r.Body, &r.CreatedAt, &r.MediaPath, &r.IpHash, &r.Number)
	if err != nil {
		return Post{}, err
	}
	return r, row.Err()
}

func PutThread(db *sql.DB, boardSlug string, subject string, body string, mediaPath string, ip_hash string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		INSERT INTO threads (board_slug, subject) 
		VALUES (?, ?) RETURNING id`, boardSlug, subject)

	if err != nil {
		return err
	}

	threadId, err := res.LastInsertId()
	if err != nil {
		return err
	}

	if err := PutPost(tx, boardSlug, int(threadId), body, mediaPath, ip_hash); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func PutPost(db Queryer, boardSlug string, threadId int, body string, mediaPath string, ip_hash string) error {
	row := db.QueryRow(`
		SELECT MAX(p.number)
		FROM posts p 
		INNER JOIN threads t ON p.thread_id = t.id
		WHERE t.board_slug = ?`, boardSlug)

	var latestPostNumber sql.NullInt64
	if err := row.Scan(&latestPostNumber); err != nil {
		return err
	}

	newPostNumber := 1
	if latestPostNumber.Valid {
		newPostNumber = int(latestPostNumber.Int64) + 1
	}

	_, err := db.Exec(`
		INSERT INTO posts (thread_id, body, media_path, ip_hash, number) 
		VALUES (?, ?, ?, ?, ?)`, threadId, body, mediaPath, ip_hash, newPostNumber)

	if err != nil {
		return err
	}

	return nil
}
