package database

import (
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path"
	"time"

	"github.com/dominicf2001/comfychan/internal/util"
)

type Queryer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
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
		SELECT id, thread_id, author, body, created_at, media_path, 
			   ip_hash, number, thumb_path, banned 
		FROM posts 
		WHERE thread_id = ?`, threadId)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Post
	for rows.Next() {
		var p Post
		err := rows.Scan(
			&p.Id, &p.ThreadId, &p.Author, &p.Body, &p.CreatedAt, &p.MediaPath,
			&p.IpHash, &p.Number, &p.ThumbPath, &p.Banned)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}

	return result, rows.Err()
}

func GetPost(db *sql.DB, postId int) (Post, error) {
	row := db.QueryRow(`
		SELECT id, thread_id, author, body, created_at, media_path, 
			   ip_hash, number, thumb_path, banned
		FROM posts 
		WHERE id = ?`, postId)

	var r Post
	err := row.Scan(
		&r.Id, &r.ThreadId, &r.Author, &r.Body, &r.CreatedAt, &r.MediaPath,
		&r.IpHash, &r.Number, &r.ThumbPath, &r.Banned)
	if err != nil {
		return Post{}, err
	}
	return r, row.Err()
}

func GetOriginalPost(db *sql.DB, threadId int) (Post, error) {
	row := db.QueryRow(`
		SELECT id, thread_id, author, body, created_at, media_path, 
			   ip_hash, number, thumb_path, banned
		FROM posts 
		WHERE thread_id = ? 
		ORDER BY created_at ASC LIMIT 1`, threadId)

	var r Post
	err := row.Scan(
		&r.Id, &r.ThreadId, &r.Author, &r.Body, &r.CreatedAt, &r.MediaPath,
		&r.IpHash, &r.Number, &r.ThumbPath, &r.Banned)
	if err != nil {
		return Post{}, err
	}
	return r, row.Err()
}

func PutThread(db *sql.DB, boardSlug string, subject string, body string, mediaPath string, thumbPath string, ip_hash string) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return -1, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		INSERT INTO threads (board_slug, subject) 
		VALUES (?, ?) RETURNING id`, boardSlug, subject)

	if err != nil {
		return -1, err
	}

	threadIdStr, err := res.LastInsertId()
	if err != nil {
		return -1, err
	}

	if err := PutPost(tx, boardSlug, int(threadIdStr), body, mediaPath, thumbPath, ip_hash); err != nil {
		return -1, err
	}

	row := tx.QueryRow("SELECT COUNT(*) FROM threads WHERE board_slug = ?", boardSlug)

	var threadCount int
	if err = row.Scan(&threadCount); err != nil {
		return -1, err
	}

	if threadCount > util.MAX_THREAD_COUNT {
		row := tx.QueryRow(`
			SELECT id FROM threads
			WHERE id = (
				SELECT id FROM threads
				WHERE board_slug = ?
				ORDER BY bumped_at ASC
				LIMIT 1)`, boardSlug)

		var pruneThreadId int
		err := row.Scan(&pruneThreadId)
		if err != nil {
			return -1, err
		}

		if err := DeleteThread(tx, pruneThreadId); err != nil {
			return -1, err
		}
	}

	if err = tx.Commit(); err != nil {
		return -1, err
	}

	return int(threadIdStr), nil
}

func PutPost(db Queryer, boardSlug string, threadId int, body string, mediaPath string, thumbPath string, ip_hash string) error {
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
		INSERT INTO posts (thread_id, body, media_path, ip_hash, number, thumb_path) 
		VALUES (?, ?, ?, ?, ?, ?)`, threadId, body, mediaPath, ip_hash, newPostNumber, thumbPath)
	if err != nil {
		return err
	}

	_, err = db.Exec(`UPDATE threads SET bumped_at = CURRENT_TIMESTAMP where id = ?`, threadId)
	if err != nil {
		return err
	}

	return nil
}

func DeleteThread(db Queryer, threadId int) error {
	// cleanup images
	rows, err := db.Query(`
		SELECT media_path, thumb_path
		FROM posts
		WHERE thread_id = ?`, threadId)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			pruneMediaThumbPath string
			pruneMediaFullPath  string
		)
		if err := rows.Scan(&pruneMediaFullPath, &pruneMediaThumbPath); err != nil {
			return err
		}

		if pruneMediaFullPath != "" {
			if err := os.Remove(path.Join(util.POST_MEDIA_FULL_PATH, pruneMediaFullPath)); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					return err
				}
			}
		}
		if pruneMediaThumbPath != "" {
			if err := os.Remove(path.Join(util.POST_MEDIA_THUMB_PATH, pruneMediaThumbPath)); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					return err
				}
			}
		}
	}

	// delete thread
	_, err = db.Exec(`
		DELETE FROM threads
		WHERE id = ?`, threadId)
	if err != nil {
		return err
	}

	return nil
}

func DeletePost(db *sql.DB, postId int) error {
	// cleanup images
	row := db.QueryRow(`
		SELECT media_path, thumb_path
		FROM posts
		WHERE id = ?`, postId)

	var (
		pruneMediaThumbPath string
		pruneMediaFullPath  string
	)
	if err := row.Scan(&pruneMediaFullPath, &pruneMediaThumbPath); err != nil {
		return err
	}

	if pruneMediaFullPath != "" {
		if err := os.Remove(path.Join(util.POST_MEDIA_FULL_PATH, pruneMediaFullPath)); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}

	if pruneMediaThumbPath != "" {
		if err := os.Remove(path.Join(util.POST_MEDIA_THUMB_PATH, pruneMediaThumbPath)); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}

	// delete post
	_, err := db.Exec(`
		DELETE FROM posts 
		WHERE id = ?`, postId)
	if err != nil {
		return err
	}

	return nil
}

func GetAdmin(db *sql.DB, username string) (Admin, error) {
	row := db.QueryRow(`
		SELECT username, password
		FROM admins
		WHERE username = ?`, username)

	var result Admin
	if err := row.Scan(&result.Username, &result.Password); err != nil {
		return Admin{}, nil
	}

	return result, nil
}

func BanIp(db *sql.DB, ip string, reason string, expiration time.Time) error {
	_, err := db.Exec(`
		INSERT INTO bans (ip_hash, reason, expiration)
		VALUES (?, ?, ?)`, ip, reason, expiration)
	return err
}
