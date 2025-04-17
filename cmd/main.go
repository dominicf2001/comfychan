package main

import (
	"fmt"
	"net/http"

	"github.com/dominicf2001/comfychan/web/views"
	"github.com/dominicf2001/comfychan/web/views/boards"
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
	http.Handle("/static/",
		disableCacheInDevMode(
			http.StripPrefix("/static",
				http.FileServer(http.Dir("web/static")))))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		views.Index().Render(r.Context(), w)
	})

	http.HandleFunc("/comfy", func(w http.ResponseWriter, r *http.Request) {
		boards.Comfy().Render(r.Context(), w)
	})

	fmt.Println("Listening on :8080")
	http.ListenAndServe(":8080", nil)
}
