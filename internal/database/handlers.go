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
