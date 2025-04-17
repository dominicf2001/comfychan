package main

import (
	"database/sql"
	"fmt"
	"github.com/dominicf2001/comfychan/internal/database"
	"github.com/dominicf2001/comfychan/web/views"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
)

var dev = true

func disableCacheInDevMode(next http.Handler) http.Handler {
	if !dev {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func main() {
	db, err := sql.Open("sqlite3", "internal/database/comfychan.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	http.Handle("/static/",
		disableCacheInDevMode(
			http.StripPrefix("/static",
				http.FileServer(http.Dir("web/static")))))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		views.Index().Render(r.Context(), w)
	})

	http.HandleFunc("/comfy", func(w http.ResponseWriter, r *http.Request) {
		board, err := database.GetBoard(db, "comfy")
		if err != nil {
			http.Error(w, "Failed to load boards", http.StatusInternalServerError)
			log.Printf("Error fetching boards: %v", err)
			return
		}

		views.Board(board).Render(r.Context(), w)
	})

	fmt.Println("Listening on :8080")
	http.ListenAndServe(":8080", nil)
}
