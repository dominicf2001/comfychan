package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dominicf2001/comfychan/internal/database"
	"github.com/dominicf2001/comfychan/internal/util"
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

func SumUniquePostIps(posts []database.Post) int {
	uniqueIpHashes := map[string]bool{}
	for _, post := range posts {
		uniqueIpHashes[post.IpHash] = true
	}
	return len(uniqueIpHashes)
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
		ip := util.GetIP(r)

		timeRemaining := util.IsOnCooldown(ip, util.ThreadCooldowns, util.THREAD_COOLDOWN)
		if timeRemaining > 0 {
			response := fmt.Sprintf("Please wait %.0f seconds.", timeRemaining.Seconds())

			http.Error(w, response, http.StatusTooManyRequests)
			return
		}

		if err := r.ParseMultipartForm(util.FILE_MEM_LIMIT); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			log.Printf("ParseMultipartForm: %v", err)
			return
		}

		subject := r.FormValue("subject")
		body := strings.TrimSpace(r.FormValue("body"))

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Failed to retrive file from form", http.StatusBadRequest)
			log.Printf("FormFile: %v", err)
			return
		}
		defer file.Close()

		filename := strconv.FormatInt(time.Now().UnixNano(), 10) + filepath.Ext(header.Filename)
		err, savedMediaPath, savedThumbPath := util.SavePostFile(file, filename)
		if err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			log.Printf("savePostFile: %v", err)
			return
		}

		if err := database.PutThread(db, slug, subject, body, savedMediaPath, savedThumbPath, util.HashIp(ip)); err != nil {
			http.Error(w, "Failed to create thread", http.StatusInternalServerError)
			log.Printf("PutThread: %v", err)
			return
		}
	})

	// CREATE POST
	r.Post("/{slug}/threads/{threadId}", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		ip := util.GetIP(r)

		timeRemaining := util.IsOnCooldown(ip, util.PostCooldowns, util.POST_COOLDOWN)
		if timeRemaining > 0 {
			response := fmt.Sprintf("Please wait %.0f seconds.", timeRemaining.Seconds())
			http.Error(w, response, http.StatusTooManyRequests)
			return
		}

		if err := r.ParseMultipartForm(util.FILE_MEM_LIMIT); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			log.Printf("ParseMultipartForm: %v", err)
			return
		}

		body := strings.TrimSpace(r.FormValue("body"))
		mediaPath := ""
		thumbPath := ""

		if body == "" {
			http.Error(w, "Malformed post body", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			if !errors.Is(err, http.ErrMissingFile) {
				http.Error(w, "Failed to retrieve file from form", http.StatusBadRequest)
				log.Printf("FormFile: %v", err)
				return
			}
		} else {
			defer file.Close()

			isFileVideo := false
			fileExt := strings.ToLower(filepath.Ext(header.Filename))

			// file type
			buffer := make([]byte, 512)
			file.Read(buffer)
			file.Seek(0, 0)
			fileType := http.DetectContentType(buffer)

			isFileVideo = strings.HasPrefix(fileType, "video/")

			if isFileVideo && !slices.Contains(util.SUPPORTED_VID_FORMATS, fileExt) {
				http.Error(w, "Unsupported file format", http.StatusBadRequest)
				return
			}

			filename := strconv.FormatInt(time.Now().UnixNano(), 10) + fileExt
			err, savedMediaPath, savedThumbPath := util.SavePostFile(file, filename)
			if err != nil {
				http.Error(w, "Failed to save file", http.StatusInternalServerError)
				log.Printf("savePostFile: %v", err)
				return
			}
			mediaPath = savedMediaPath
			thumbPath = savedThumbPath
		}

		threadIdStr := chi.URLParam(r, "threadId")
		threadId, err := strconv.Atoi(threadIdStr)
		if err != nil {
			http.Error(w, "Invalid thread id", http.StatusBadRequest)
			return
		}

		if err := database.PutPost(db, slug, threadId, body, mediaPath, thumbPath, util.HashIp(ip)); err != nil {
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
				log.Printf("GetOriginalPost: %v", err)
				return
			}
			op := posts[0]

			if op.MediaPath == "" {
				http.Error(w, "Malformed thread", http.StatusInternalServerError)
				log.Printf("Thread %d has no OP image", thread.Id)
				return
			}

			previews = append(previews, views.CatalogThreadPreview{
				Subject:    thread.Subject,
				Body:       op.Body,
				ThreadURL:  fmt.Sprintf("/%s/threads/%d", slug, thread.Id),
				ThumbPath:  op.ThumbPath,
				ReplyCount: len(posts),
				IpCount:    SumUniquePostIps(posts),
			})
		}

		views.ThreadsCatalog(previews).Render(r.Context(), w)
	})

	// THREAD POSTS
	r.Get("/hx/{slug}/threads/{threadId}/posts", func(w http.ResponseWriter, r *http.Request) {
		// slug := chi.URLParam(r, "slug")
		threadIdStr := chi.URLParam(r, "threadId")
		threadId, err := strconv.Atoi(threadIdStr)
		if err != nil {
			http.Error(w, "Invalid thread id", http.StatusBadRequest)
			return
		}

		thread, err := database.GetThread(db, threadId)
		if err != nil {
			http.Error(w, "Failed to get thread", http.StatusBadRequest)
			log.Printf("GetThread: %v", err)
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
		views.Posts(posts, thread).Render(r.Context(), w)
	})

	// -----------------

	// -----------------
	// CLEANUP
	// -----------------

	go func() {
		for range time.Tick(10 * time.Minute) {
			util.CooldownMutex.Lock()
			defer util.CooldownMutex.Unlock()
			for ip, cooldown := range util.PostCooldowns {
				if time.Since(cooldown) >= util.POST_COOLDOWN {
					delete(util.PostCooldowns, ip)
				}
			}

			for ip, cooldown := range util.ThreadCooldowns {
				if time.Since(cooldown) >= util.THREAD_COOLDOWN {
					delete(util.ThreadCooldowns, ip)
				}
			}
		}
	}()

	// -----------------

	fmt.Println("Listening on :8080")
	http.ListenAndServe(":8080", r)
}
