package main

import (
	"database/sql"
	"fmt"
	"github.com/dominicf2001/comfychan/internal/database"
	"github.com/dominicf2001/comfychan/web/views"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
			log.Printf("Board %q not found: %v", slug, err)
			return
		}

		views.Board(board).Render(r.Context(), w)
	})

	// r.Get("/{slug}/{threadId}/posts", func(w http.ResponseWriter, r *http.Request) {
	// 	slug := chi.URLParam(r, "slug")
	// 	threadIdStr := chi.URLParam(r, "postId")
	// 	threadId, err := strconv.Atoi(threadIdStr)
	// 	if err != nil {
	// 		http.Error(w, "Invalid thread id", http.StatusBadRequest)
	// 		return
	// 	}
	//
	// 	posts, err := database.GetThreadPosts(db, threadId)
	// 	if err != nil {
	// 		http.Error(w, "Failed to get thread posts", http.StatusInternalServerError)
	// 	}
	// })

	r.Get("/hx/{slug}/catalog", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		threads, err := database.GetBoardThreads(db, slug)
		if err != nil {
			http.Error(w, "Failed to get board threads", http.StatusInternalServerError)
			log.Printf("GetBoardThreads: %v", err)
			return
		}

		vms := make([]views.ThreadGridBoxViewModel, 0, len(threads))
		for _, thread := range threads {
			posts, err := database.GetThreadPosts(db, thread.Id)
			if err != nil {
				http.Error(w, "Error getting thread posts", http.StatusBadRequest)
				log.Printf("GetThreadPosts: %v", err)
				return
			}

			vms = append(vms, views.ThreadGridBoxViewModel{
				Thread: thread,
				Posts:  posts,
			})
		}

		views.PostsCatalog(vms).Render(r.Context(), w)
	})

	r.Post("/{slug}/threads", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad form data", http.StatusBadRequest)
			return
		}

		subject := r.FormValue("subject")
		body := r.FormValue("body")

		database.PutBoardThread(db, slug, subject, body)
	})

	fmt.Println("Listening on :8080")
	http.ListenAndServe(":8080", r)
}
