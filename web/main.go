package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"

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
	// -----------------
	// SETUP
	// -----------------

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

	// -----------------

	// -----------------
	// MAIN ROUTES
	// -----------------

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

	r.Get("/{slug}/threads/{threadId}", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		// TODO: use relative thread_nums (see issue #1)
		threadIdStr := chi.URLParam(r, "threadId")
		threadId, err := strconv.Atoi(threadIdStr)
		if err != nil {
			http.Error(w, "Invalid thread id", http.StatusBadRequest)
			return
		}

		board, err := database.GetBoard(db, slug)
		if err != nil {
			http.NotFound(w, r)
			log.Printf("Board %q not found: %v", slug, err)
			return
		}

		thread, err := database.GetThread(db, threadId)
		if err != nil {
			http.Error(w, "Failed to get thread", http.StatusInternalServerError)
			log.Printf("Failed to get thread %q: %v", threadIdStr, err)
			return
		}

		posts, err := database.GetThreadPosts(db, threadId)
		if err != nil {
			http.Error(w, "Failed to get thread posts", http.StatusInternalServerError)
			log.Printf("Failed to get thread %q posts: %v", threadIdStr, err)
			return
		}

		if len(posts) == 0 {
			http.Error(w, "Malformed thread", http.StatusInternalServerError)
			log.Printf("Thread %d has no posts", thread.Id)
			return
		}

		views.Thread(board, thread, posts).Render(r.Context(), w)
	})

	r.Post("/{slug}/threads/{threadId}", func(w http.ResponseWriter, r *http.Request) {
		// slug := chi.URLParam(r, "slug")
		// TODO: use relative thread_nums (see issue #1)
		threadIdStr := chi.URLParam(r, "threadId")
		threadId, err := strconv.Atoi(threadIdStr)
		if err != nil {
			http.Error(w, "Invalid thread id", http.StatusBadRequest)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad form data", http.StatusBadRequest)
			log.Printf("ParseForm: %v", err)
			return
		}

		body := r.FormValue("body")

		if err := database.PutPost(db, threadId, body); err != nil {
			http.Error(w, "Failed to create post", http.StatusInternalServerError)
			log.Printf("PutPost: %v", err)
			return
		}
	})

	r.Post("/{slug}/threads", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad form data", http.StatusBadRequest)
			log.Printf("ParseForm: %v", err)
			return
		}

		subject := r.FormValue("subject")
		body := r.FormValue("body")

		if err := database.PutThread(db, slug, subject, body); err != nil {
			http.Error(w, "Failed to create thread", http.StatusInternalServerError)
			log.Printf("PutThread: %v", err)
			return
		}
	})

	// -----------------

	// -----------------
	// PARTIALS
	// -----------------

	r.Get("/hx/{slug}/catalog", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		threads, err := database.GetThreads(db, slug)
		if err != nil {
			http.Error(w, "Failed to get board threads", http.StatusInternalServerError)
			log.Printf("GetThreads: %v", err)
			return
		}

		previews := make([]views.CatalogThreadPreview, 0, len(threads))
		for _, thread := range threads {
			posts, err := database.GetThreadPosts(db, thread.Id)
			if err != nil {
				http.Error(w, "Error getting thread posts", http.StatusBadRequest)
				log.Printf("GetThreadPosts: %v", err)
				return
			}

			if len(posts) == 0 {
				http.Error(w, "Malformed thread", http.StatusInternalServerError)
				log.Printf("Thread %d has no posts", thread.Id)
				return
			}

			previews = append(previews, views.CatalogThreadPreview{
				Subject:   thread.Subject,
				Body:      posts[0].Body,
				ThreadURL: fmt.Sprintf("/%s/threads/%d", slug, thread.Id),
			})
		}

		views.ThreadsCatalog(previews).Render(r.Context(), w)
	})

	// -----------------

	fmt.Println("Listening on :8080")
	http.ListenAndServe(":8080", r)
}
