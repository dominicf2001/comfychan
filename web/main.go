package main

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/dominicf2001/comfychan/internal/database"
	"github.com/dominicf2001/comfychan/internal/util"
	"github.com/dominicf2001/comfychan/web/views"
	"github.com/dominicf2001/comfychan/web/views/admin"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func AdminOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("comfy_admin")
		if err != nil || !util.IsAdminSessionValid(c.Value) {
			// Check if it's an HTMX request
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/authorize")
				w.WriteHeader(http.StatusOK)
			} else {
				http.Redirect(w, r, "/authorize", http.StatusSeeOther)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func disableCacheInDevMode(next http.Handler) http.Handler {
	if !util.DevMode {
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

func isAdmin(r *http.Request) bool {
	if util.DevMode {
		return true
	}

	admin := false
	if c, err := r.Cookie("comfy_admin"); err == nil {
		admin = util.IsAdminSessionValid(c.Value)
	}
	return admin
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

		views.Board(board, isAdmin(r)).Render(r.Context(), w)
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

		views.Thread(board, thread, posts, views.ThreadContext{
			IsAdmin: isAdmin(r),
		}).Render(r.Context(), w)
	})

	// CREATE THREAD
	r.Post("/{slug}/threads", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		ip := util.GetIP(r)

		// check cooldown
		timeRemaining := util.GetRemainingCooldown(ip, util.ThreadCooldowns, util.THREAD_COOLDOWN)
		if timeRemaining > 0 && !isAdmin(r) {
			response := fmt.Sprintf("Please wait %.0f seconds", timeRemaining.Seconds())
			io.Copy(io.Discard, r.Body)
			http.Error(w, response, http.StatusTooManyRequests)
			return
		}

		// parse form
		r.Body = http.MaxBytesReader(w, r.Body, util.MAX_REQUEST_BYTES)
		if err := r.ParseMultipartForm(util.FILE_MEM_LIMIT); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				http.Error(w, fmt.Sprintf("File too large (max %s)", util.FormatBytes(util.FILE_MEM_LIMIT)), http.StatusRequestEntityTooLarge)
				return
			}

			if errors.Is(err, multipart.ErrMessageTooLarge) {
				http.Error(w, fmt.Sprintf("File too large (max %s)", util.FormatBytes(util.FILE_MEM_LIMIT)), http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			log.Printf("ParseMultipartForm: %v", err)
			return
		}

		// validate inputs
		subject := strings.TrimSpace(r.FormValue("subject"))
		body := strings.TrimSpace(r.FormValue("body"))

		if len(subject) > util.MAX_SUBJECT_LEN {
			http.Error(w, fmt.Sprintf("Subject exceeds %d characters", util.MAX_SUBJECT_LEN), http.StatusBadRequest)
			return
		}

		if body == "" {
			http.Error(w, "Body is empty", http.StatusBadRequest)
			return
		}

		if len(body) > util.MAX_BODY_LEN {
			http.Error(w, fmt.Sprintf("Body exceeds %d characters", util.MAX_BODY_LEN), http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Failed to retrive file from form", http.StatusBadRequest)
			log.Printf("FormFile: %v", err)
			return
		}
		defer file.Close()

		if header.Size > util.FILE_MEM_LIMIT {
			http.Error(w, fmt.Sprintf("File too large (max %s)", util.FormatBytes(util.FILE_MEM_LIMIT)), http.StatusRequestEntityTooLarge)
			return
		}

		mediaType, err := util.DetectPostFileType(file)
		if err != nil {
			http.Error(w, "Failed to detect if file is a video", http.StatusInternalServerError)
			return
		}
		file.Seek(0, io.SeekStart)

		if mediaType == util.PostFileUnsupported {
			http.Error(w, "Unsupported media type", http.StatusBadRequest)
			return
		}

		filename := strconv.FormatInt(time.Now().UnixNano(), 10) + filepath.Ext(header.Filename)
		err, savedMediaPath, savedThumbPath := util.SavePostFile(file, filename)
		if err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			log.Printf("savePostFile: %v", err)
			return
		}

		threadId, err := database.PutThread(db, slug, subject, body, savedMediaPath, savedThumbPath, util.HashIp(ip))
		if err != nil {
			http.Error(w, "Failed to create thread", http.StatusInternalServerError)
			log.Printf("PutThread: %v", err)
			return
		}

		util.BeginCooldown(ip, util.ThreadCooldowns, util.THREAD_COOLDOWN)

		// Check if it's an HTMX request
		redirectUrl := fmt.Sprintf("/%s/threads/%d", slug, threadId)
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", redirectUrl)
			w.WriteHeader(http.StatusOK)
		} else {
			http.Redirect(w, r, redirectUrl, http.StatusSeeOther)
		}
	})

	// CREATE POST
	r.Post("/{slug}/threads/{threadId}", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		ip := util.GetIP(r)

		// check cooldown
		timeRemaining := util.GetRemainingCooldown(ip, util.PostCooldowns, util.POST_COOLDOWN)
		if timeRemaining > 0 && !isAdmin(r) {
			response := fmt.Sprintf("Please wait %.0f seconds", timeRemaining.Seconds())
			io.Copy(io.Discard, r.Body)
			http.Error(w, response, http.StatusTooManyRequests)
			return
		}

		// parse form
		r.Body = http.MaxBytesReader(w, r.Body, util.MAX_REQUEST_BYTES)
		if err := r.ParseMultipartForm(util.FILE_MEM_LIMIT); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				http.Error(w, fmt.Sprintf("File too large (max %s)", util.FormatBytes(util.FILE_MEM_LIMIT)), http.StatusRequestEntityTooLarge)
				return
			}

			if errors.Is(err, multipart.ErrMessageTooLarge) {
				http.Error(w, fmt.Sprintf("File too large (max %s)", util.FormatBytes(util.FILE_MEM_LIMIT)), http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			log.Printf("ParseMultipartForm: %v", err)
			return
		}

		// validate inputs
		body := strings.TrimSpace(r.FormValue("body"))
		mediaPath := ""
		thumbPath := ""

		if body == "" {
			http.Error(w, "Body is empty", http.StatusBadRequest)
			return
		}

		if len(body) > util.MAX_BODY_LEN {
			http.Error(w, fmt.Sprintf("Body exceeds %d characters", util.MAX_BODY_LEN), http.StatusBadRequest)
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

			if header.Size > util.FILE_MEM_LIMIT {
				http.Error(w, fmt.Sprintf("File too large (max %s)", util.FormatBytes(util.FILE_MEM_LIMIT)), http.StatusRequestEntityTooLarge)
				return
			}

			mediaType, err := util.DetectPostFileType(file)
			if err != nil {
				http.Error(w, "Failed to detect if file is a video", http.StatusInternalServerError)
				return
			}
			file.Seek(0, io.SeekStart)

			if mediaType == util.PostFileUnsupported {
				http.Error(w, "Unsupported media type", http.StatusBadRequest)
				return
			}

			filename := strconv.FormatInt(time.Now().UnixNano(), 10) + filepath.Ext(header.Filename)
			err, savedMediaPath, savedThumbPath := util.SavePostFile(file, filename)
			if err != nil {
				http.Error(w, "Failed to save file", http.StatusInternalServerError)
				log.Printf("savePostFile: %v", err)
				return
			}
			mediaPath = savedMediaPath
			thumbPath = savedThumbPath
		}

		// put post into DB
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

		util.BeginCooldown(ip, util.PostCooldowns, util.POST_COOLDOWN)
	})

	// -----------------

	// -----------------
	// PARTIAL ROUTES (htmx)
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
				ThreadId:   thread.Id,
				ThreadURL:  fmt.Sprintf("/%s/threads/%d", slug, thread.Id),
				ThumbPath:  op.ThumbPath,
				ReplyCount: len(posts),
				IpCount:    SumUniquePostIps(posts),
			})
		}

		views.ThreadsCatalog(previews, views.CatalogContext{
			IsAdmin:   isAdmin(r),
			BoardSlug: slug,
		}).Render(r.Context(), w)
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
		views.Posts(posts, thread, views.ThreadContext{
			IsAdmin: isAdmin(r),
		}).Render(r.Context(), w)
	})

	// -----------------

	// -----------------
	// ADMIN ROUTES (htmx)
	// -----------------

	r.Get("/authorize", func(w http.ResponseWriter, r *http.Request) {
		admin.AdminLogin().Render(r.Context(), w)
	})

	r.Post("/authorize", func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		password := r.FormValue("password")

		admin, err := database.GetAdmin(db, username)
		if err != nil {
			http.Error(w, "Invalid login", http.StatusUnauthorized)
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(password))
		if err != nil {
			http.Error(w, "Invalid login", http.StatusUnauthorized)
			return
		}

		token, err := util.GenToken()
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		tokenValidUntil := time.Now().Add(time.Hour)
		util.CreateAdminSession(token, util.AdminSession{
			Username:   username,
			Expiration: tokenValidUntil,
		})

		http.SetCookie(w, &http.Cookie{
			Name:     "comfy_admin",
			Value:    token,
			HttpOnly: true,
			Secure:   !util.DevMode,
			Expires:  tokenValidUntil,
			SameSite: http.SameSiteStrictMode,
			Path:     "/",
		})

	})

	r.Get("/authorized", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		if c, err := r.Cookie("comfy_admin"); err == nil && util.IsAdminSessionValid(c.Value) {
			w.Write([]byte("true"))
		} else {
			w.Write([]byte("false"))
		}
	})

	r.Route("/admin", func(r chi.Router) {
		r.Use(AdminOnlyMiddleware)

		r.Post("/logout", func(w http.ResponseWriter, r *http.Request) {
			if c, err := r.Cookie("comfy_admin"); err == nil {
				util.DeleteAdminSession(c.Value)
			}
			http.SetCookie(w, &http.Cookie{
				Name:     "comfy_admin",
				Value:    "",
				HttpOnly: true,
				Secure:   !util.DevMode,
				Expires:  time.Now(),
				SameSite: http.SameSiteStrictMode,
				Path:     "/",
			})
		})

		r.Delete("/threads/{threadId}", func(w http.ResponseWriter, r *http.Request) {
			threadIdStr := chi.URLParam(r, "threadId")
			threadId, err := strconv.Atoi(threadIdStr)
			if err != nil {
				http.Error(w, "Invalid thread id", http.StatusBadRequest)
				return
			}

			err = database.DeleteThread(db, threadId)
			if err != nil {
				log.Println("DeleteThread: ", err)
				http.Error(w, "Failed to delete thread: "+threadIdStr, http.StatusInternalServerError)
				return
			}
		})

		r.Delete("/posts/{postId}", func(w http.ResponseWriter, r *http.Request) {
			postIdStr := chi.URLParam(r, "postId")
			postId, err := strconv.Atoi(postIdStr)
			if err != nil {
				http.Error(w, "Invalid post id", http.StatusBadRequest)
				return
			}

			err = database.DeletePost(db, postId)
			if err != nil {
				log.Println("DeletePost: ", err)
				http.Error(w, "Failed to delete post: "+postIdStr, http.StatusInternalServerError)
				return
			}
		})

		// bans the ip stored in the post id
		r.Post("/ban/{postId}", func(w http.ResponseWriter, r *http.Request) {
			postIdStr := chi.URLParam(r, "postId")
			postId, err := strconv.Atoi(postIdStr)
			if err != nil {
				http.Error(w, "Invalid post id", http.StatusBadRequest)
				return
			}

			_, err = db.Exec(`UPDATE posts SET banned = 1 WHERE id = ?`, postId)
			if err != nil {
				http.Error(w, "Failed update post to banned", http.StatusInternalServerError)
				return
			}

			post, err := database.GetPost(db, postId)
			if err != nil {
				log.Println("GetPost: ", err)
				http.Error(w, "Failed to get post: "+postIdStr, http.StatusInternalServerError)
				return
			}

			ipToBan := post.IpHash
			reason := r.FormValue("reason")

			expirationInput := r.FormValue("expiration")
			expiration, err := time.Parse("2006-01-02T15:04", expirationInput)
			if err != nil {
				http.Error(w, "Invalid expiration datetime value", http.StatusBadRequest)
				return
			}

			err = database.BanIp(db, ipToBan, reason, expiration)
			if err != nil {
				log.Println("BanIp: ", err)
				http.Error(w, "Failed to ban ip: ", http.StatusInternalServerError)
				return
			}
		})
	})

	// -----------------

	// -----------------
	// CLEANUP
	// -----------------

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			util.CooldownMutex.Lock()
			for ip, t := range util.PostCooldowns {
				if time.Since(t) >= util.POST_COOLDOWN {
					delete(util.PostCooldowns, ip)
				}
			}
			for ip, t := range util.ThreadCooldowns {
				if time.Since(t) >= util.THREAD_COOLDOWN {
					delete(util.ThreadCooldowns, ip)
				}
			}
			util.CooldownMutex.Unlock()
		}
	}()

	// -----------------

	fmt.Println("Listening on :8080")
	http.ListenAndServe(":8080", r)
}
