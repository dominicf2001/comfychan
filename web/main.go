package main

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/dominicf2001/comfychan/internal/database"
	"github.com/dominicf2001/comfychan/web/views"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
)

const FILE_MEM_LIMIT int64 = 10 << 20

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

func savePostFile(file *multipart.File, header *multipart.FileHeader) error {
	dstPath := filepath.Join("web/static/img/posts", header.Filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		log.Printf("os.Create: %v", err)
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, *file); err != nil {
		log.Printf("io.Copy: %v", err)
		return err
	}

	return nil
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

	// INDEX PAGE
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		views.Index().Render(r.Context(), w)
	})

	// MAIN BOARD PAGE
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

	// THREAD PAGE
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

		posts, err := database.GetPosts(db, threadId)
		if err != nil {
			http.Error(w, "Failed to get posts", http.StatusInternalServerError)
			log.Printf("Failed to get thread %q posts: %v", threadIdStr, err)
			return
		}

		if len(posts) == 0 || posts[0].MediaPath == "" {
			http.Error(w, "Malformed thread", http.StatusInternalServerError)
			log.Printf("Thread %d has no posts or no OP image", threadId)
			return
		}

		views.Thread(board, thread, posts).Render(r.Context(), w)
	})

	// CREATE THREAD
	r.Post("/{slug}/threads", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")

		subject := r.FormValue("subject")
		body := r.FormValue("body")

		// 10 MB memory limit
		if err := r.ParseMultipartForm(FILE_MEM_LIMIT); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			log.Printf("ParseMultipartForm: %v", err)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Failed to retrive file from form", http.StatusBadRequest)
			log.Printf("FormFile: %v", err)
			return
		}
		defer file.Close()

		if err := savePostFile(&file, header); err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			log.Printf("savePostFile: %v", err)
			return
		}

		if err := database.PutThread(db, slug, subject, body, header.Filename); err != nil {
			http.Error(w, "Failed to create thread", http.StatusInternalServerError)
			log.Printf("PutThread: %v", err)
			return
		}
	})

	// CREATE POST
	r.Post("/{slug}/threads/{threadId}", func(w http.ResponseWriter, r *http.Request) {
		// slug := chi.URLParam(r, "slug")
		// TODO: use relative thread_nums (see issue #1)
		threadIdStr := chi.URLParam(r, "threadId")
		threadId, err := strconv.Atoi(threadIdStr)
		if err != nil {
			http.Error(w, "Invalid thread id", http.StatusBadRequest)
			return
		}

		body := r.FormValue("body")
		mediaPath := ""

		// 10 MB memory limit
		if err := r.ParseMultipartForm(FILE_MEM_LIMIT); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			log.Printf("ParseMultipartForm: %v", err)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			if !errors.Is(err, http.ErrMissingFile) {
				http.Error(w, "Failed to retrive file from form", http.StatusBadRequest)
				log.Printf("FormFile: %v", err)
				return
			}
		} else {
			defer file.Close()
			if err := savePostFile(&file, header); err != nil {
				http.Error(w, "Failed to save file", http.StatusInternalServerError)
				log.Printf("savePostFile: %v", err)
				return
			}
			mediaPath = header.Filename
		}

		if err := database.PutPost(db, threadId, body, mediaPath); err != nil {
			http.Error(w, "Failed to create post", http.StatusInternalServerError)
			log.Printf("PutPost: %v", err)
			return
		}
	})

	// -----------------

	// -----------------
	// PARTIALS (htmx)
	// -----------------

	// CATALOG
	r.Get("/hx/{slug}/catalog", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		threads, err := database.GetThreads(db, slug)
		if err != nil {
			http.Error(w, "Failed to get threads", http.StatusInternalServerError)
			log.Printf("GetThreads: %v", err)
			return
		}

		previews := make([]views.CatalogThreadPreview, 0, len(threads))
		for _, thread := range threads {
			posts, err := database.GetPosts(db, thread.Id)
			if err != nil {
				http.Error(w, "Failed to get posts", http.StatusBadRequest)
				log.Printf("GetPosts: %v", err)
				return
			}

			if len(posts) == 0 || posts[0].MediaPath == "" {
				http.Error(w, "Malformed thread", http.StatusInternalServerError)
				log.Printf("Thread %d has no posts or no OP image", thread.Id)
				return
			}

			previews = append(previews, views.CatalogThreadPreview{
				Subject:   thread.Subject,
				Body:      posts[0].Body,
				ThreadURL: fmt.Sprintf("/%s/threads/%d", slug, thread.Id),
				MediaPath: posts[0].MediaPath,
			})
		}

		views.ThreadsCatalog(previews).Render(r.Context(), w)
	})

	// THREAD POSTS
	r.Get("/hx/{slug}/threads/{threadId}/posts", func(w http.ResponseWriter, r *http.Request) {
		// slug := chi.URLParam(r, "slug")
		// TODO: use relative thread_nums (see issue #1)
		threadIdStr := chi.URLParam(r, "threadId")
		threadId, err := strconv.Atoi(threadIdStr)
		if err != nil {
			http.Error(w, "Invalid thread id", http.StatusBadRequest)
			return
		}

		posts, err := database.GetPosts(db, threadId)
		if err != nil {
			http.Error(w, "Failed to get posts", http.StatusBadRequest)
			log.Printf("GetPosts: %v", err)
			return
		}

		if len(posts) == 0 || posts[0].MediaPath == "" {
			http.Error(w, "Malformed thread", http.StatusInternalServerError)
			log.Printf("Thread %d has no posts or no OP image", threadId)
			return
		}

		// dont pass the op post. only replies
		views.ThreadPosts(posts[1:]).Render(r.Context(), w)
	})

	// -----------------

	fmt.Println("Listening on :8080")
	http.ListenAndServe(":8080", r)
}
