package database

import (
	"database/sql"
)

func GetBoards(db *sql.DB) ([]Board, error) {
	rows, err := db.Query(`select id, name, slug, tag from boards order by slug`)
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
	row := db.QueryRow(`SELECT id, name, slug, tag FROM boards WHERE slug = ?`, slug)

	var result Board
	err := row.Scan(&result.Id, &result.Name, &result.Slug, &result.Tag)
	if err != nil {
		return Board{}, err
	}

	return result, row.Err()
}

func GetThreads(db *sql.DB, slug string) ([]Thread, error) {
	rows, err := db.Query(
		`SELECT id, board_slug, subject, created_at, bumped_at FROM threads WHERE board_slug = ?`, slug)
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
	row := db.QueryRow(`SELECT id, board_slug, subject, created_at, bumped_at FROM threads WHERE id = ?`, threadId)

	var thread Thread
	err := row.Scan(&thread.Id, &thread.BoardSlug, &thread.Subject, &thread.CreatedAt, &thread.BumpedAt)
	if err != nil {
		return Thread{}, err
	}

	return thread, row.Err()
}

func GetThreadPosts(db *sql.DB, threadId int) ([]Post, error) {
	rows, err := db.Query(`SELECT id, thread_id, author, body, created_at FROM posts where thread_id = ?`, threadId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Post
	for rows.Next() {
		var p Post
		err := rows.Scan(&p.Id, &p.ThreadId, &p.Author, &p.Body, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}

	return result, rows.Err()
}

func PutBoardThread(db *sql.DB, boardSlug string, subject string, body string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO threads (board_slug, subject) VALUES (?, ?) RETURNING id`, boardSlug, subject)
	if err != nil {
		return err
	}

	threadId, err := res.LastInsertId()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`INSERT INTO posts (thread_id, body) VALUES (?, ?)`, threadId, body)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}
