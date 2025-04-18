package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/dominicf2001/comfychan/internal/database"
	"github.com/dominicf2001/comfychan/web/views"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
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

	r := chi.NewRouter()

	r.Use(middleware.Logger)

	r.Handle("/static/*",
		disableCacheInDevMode(
			http.StripPrefix("/static",
				http.FileServer(http.Dir("web/static")))))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		views.Index().Render(r.Context(), w)
	})

	r.Get("/{slug}", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")

		board, err := database.GetBoard(db, slug)
		if err != nil {
			http.NotFound(w, r)
			log.Printf("board %q not found: %v", slug, err)
			return
		}

		views.Board(board).Render(r.Context(), w)
	})

	fmt.Println("Listening on :8080")
	http.ListenAndServe(":8080", r)
}
