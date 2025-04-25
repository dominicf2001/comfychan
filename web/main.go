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
	"strings"
	"time"

	"github.com/dominicf2001/comfychan/internal/database"
	"github.com/dominicf2001/comfychan/internal/util"
	"github.com/dominicf2001/comfychan/web/views"
	"github.com/dominicf2001/comfychan/web/views/admin"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

func AdminOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("comfy_admin")
		if !util.DevMode && (err != nil || !util.IsAdminSessionValid(c.Value)) {
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

	// init paths

	dataDir := os.Getenv("COMFYCHAN_DATA_DIR")
	if dataDir == "" {
		dataDir = "." // for development
	}

	util.POST_MEDIA_FULL_PATH = filepath.Join(dataDir, util.POST_MEDIA_FULL_PATH)
	util.POST_MEDIA_THUMB_PATH = filepath.Join(dataDir, util.POST_MEDIA_FULL_PATH)
	util.DATABASE_PATH = filepath.Join(dataDir, util.DATABASE_PATH)
	util.STATIC_PATH = filepath.Join(dataDir, util.STATIC_PATH)

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
				http.FileServer(http.Dir(util.STATIC_PATH)))))

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
			if errors.Is(err, sql.ErrNoRows) {
				views.NotFound().Render(r.Context(), w)
				return
			}
			http.Error(w, "Failed to get board", http.StatusInternalServerError)
			log.Printf("Failed to get board%q: %v", slug, err)
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
			if errors.Is(err, sql.ErrNoRows) {
				views.NotFound().Render(r.Context(), w)
				return
			}
			http.Error(w, "Failed to get board", http.StatusInternalServerError)
			log.Printf("Failed to get board%q: %v", slug, err)
			return
		}

		thread, err := database.GetThread(db, threadId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				views.NotFound().Render(r.Context(), w)
				return
			}
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
		ipHash := util.HashIp(util.GetIP(r))

		// guard banned ips
		ban, err := database.GetBan(db, ipHash)
		if err != nil {
			if !errors.Is(err, database.ErrBanNotFound) {
				http.Error(w, "Failed to get ban", http.StatusInternalServerError)
				return
			}
		} else {
			msg := fmt.Sprintf("You are banned until: %s. Reason: %s",
				ban.Expiration.Format("2006-01-02 15:04"),
				ban.Reason)
			http.Error(w, msg, http.StatusForbidden)
			return
		}

		// check cooldown
		timeRemaining := util.GetRemainingCooldown(ipHash, util.ThreadCooldowns, util.THREAD_COOLDOWN)
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

		threadId, err := database.PutThread(db, slug, subject, body, savedMediaPath, savedThumbPath, ipHash)
		if err != nil {
			http.Error(w, "Failed to create thread", http.StatusInternalServerError)
			log.Printf("PutThread: %v", err)
			return
		}

		util.BeginCooldown(ipHash, util.ThreadCooldowns, util.THREAD_COOLDOWN)

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
		ipHash := util.HashIp(util.GetIP(r))

		threadIdStr := chi.URLParam(r, "threadId")
		threadId, err := strconv.Atoi(threadIdStr)
		if err != nil {
			http.Error(w, "Invalid thread id", http.StatusBadRequest)
			return
		}

		// guard banned ips
		ban, err := database.GetBan(db, ipHash)
		if err != nil {
			if !errors.Is(err, database.ErrBanNotFound) {
				http.Error(w, "Failed to get ban", http.StatusInternalServerError)
				return
			}
		} else {
			msg := fmt.Sprintf("You are banned until: %s. Reason: %s",
				ban.Expiration.Format("2006-01-02 15:04"),
				ban.Reason)
			http.Error(w, msg, http.StatusForbidden)
			return
		}

		// check cooldown
		timeRemaining := util.GetRemainingCooldown(ipHash, util.PostCooldowns, util.POST_COOLDOWN)
		if timeRemaining > 0 && !isAdmin(r) {
			response := fmt.Sprintf("Please wait %.0f seconds", timeRemaining.Seconds())
			io.Copy(io.Discard, r.Body)
			http.Error(w, response, http.StatusTooManyRequests)
			return
		}

		// guard if thread locked
		isLocked := true
		row := db.QueryRow(`SELECT locked FROM threads where id = ?`, threadId)
		if err := row.Scan(&isLocked); err != nil {
			http.Error(w, "Failed to check if thread locked", http.StatusInternalServerError)
			return
		}

		if isLocked && !isAdmin(r) {
			http.Error(w, "This thread is locked", http.StatusForbidden)
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

		if err := database.PutPost(db, slug, threadId, body, mediaPath, thumbPath, ipHash); err != nil {
			http.Error(w, "Failed to create post", http.StatusInternalServerError)
			log.Printf("PutPost: %v", err)
			return
		}

		util.BeginCooldown(ipHash, util.PostCooldowns, util.POST_COOLDOWN)
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

			uniqueIpHashes := map[string]bool{}
			for _, post := range posts {
				uniqueIpHashes[post.IpHash] = true
			}

			previews = append(previews, views.CatalogThreadPreview{
				Subject:    thread.Subject,
				Body:       op.Body,
				ThreadId:   thread.Id,
				ThreadURL:  fmt.Sprintf("/%s/threads/%d", slug, thread.Id),
				ThumbPath:  op.ThumbPath,
				ReplyCount: len(posts),
				IpCount:    len(uniqueIpHashes),
				Pinned:     thread.Pinned,
				Locked:     thread.Locked,
				BumpedAt:   thread.BumpedAt,
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
			if errors.Is(err, sql.ErrNoRows) {
				views.NotFound().Render(r.Context(), w)
				return
			}
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

	r.Route("/admin", func(r chi.Router) {
		r.Use(AdminOnlyMiddleware)

		r.Patch("/threads/{threadId}/lock", func(w http.ResponseWriter, r *http.Request) {
			threadIdStr := chi.URLParam(r, "threadId")
			threadId, err := strconv.Atoi(threadIdStr)
			if err != nil {
				http.Error(w, "Invalid thread id", http.StatusBadRequest)
				return
			}

			lockedStr := r.URL.Query().Get("locked")
			locked, err := strconv.ParseBool(lockedStr)
			if err != nil {
				http.Error(w, "Invalid values for 'locked'", http.StatusBadRequest)
				return
			}

			_, err = db.Exec(`UPDATE threads SET locked = ? WHERE id = ?`, locked, threadId)
			if err != nil {
				log.Println("Updating thread 'locked': ", err)
				http.Error(w, "Failed to lock thread: "+threadIdStr, http.StatusInternalServerError)
				return
			}
		})

		r.Patch("/threads/{threadId}/pin", func(w http.ResponseWriter, r *http.Request) {
			threadIdStr := chi.URLParam(r, "threadId")
			threadId, err := strconv.Atoi(threadIdStr)
			if err != nil {
				http.Error(w, "Invalid thread id", http.StatusBadRequest)
				return
			}

			pinnedStr := r.URL.Query().Get("pinned")
			pinned, err := strconv.ParseBool(pinnedStr)
			if err != nil {
				http.Error(w, "Invalid values for 'pinned'", http.StatusBadRequest)
				return
			}

			_, err = db.Exec(`UPDATE threads SET pinned = ? WHERE id = ?`, pinned, threadId)
			if err != nil {
				log.Println("Updating thread 'pinned': ", err)
				http.Error(w, "Failed to pin thread: "+threadIdStr, http.StatusInternalServerError)
				return
			}
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
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		views.NotFound().Render(r.Context(), w)
	})

	// -----------------

	// -----------------
	// CLEANUP
	// -----------------

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// cleanup cooldowns
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

			// cleanup admin sessions
			util.AdminMutex.Lock()
			for token, s := range util.AdminSessions {
				if time.Now().After(s.Expiration) {
					delete(util.AdminSessions, token)
				}
			}
			util.AdminMutex.Unlock()
		}
	}()

	// -----------------

	fmt.Println("Listening on 0.0.0.0:7676")
	http.ListenAndServe("0.0.0.0:7676", r)
}
