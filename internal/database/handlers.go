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
	row.Scan(&result.Id, &result.Name, &result.Slug, &result.Tag)

	return result, row.Err()
}

func GetBoardThreads(db *sql.DB, board_id int) ([]Thread, error) {
	rows, err := db.Query(
		`SELECT id, board_id, subject, created_at, bumped_at FROM threads WHERE board_id = ?`, board_id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Thread
	for rows.Next() {
		var t Thread
		err := rows.Scan(&t.Id, &t.BoardId, &t.Subject, &t.CreatedAt, &t.BumpedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}

	return result, rows.Err()
}

func GetThreadPosts(db *sql.DB, thread_id int) ([]Post, error) {
	rows, err := db.Query(`SELECT id, thread_id, author, body, created_at FROM posts where thread_id = ?`, thread_id)
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

func PutBoardThread(db *sql.DB, boardId int, subject string, body string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO threads (board_id, subject) VALUES (?, ?) RETURNING id`, boardId, subject)
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
