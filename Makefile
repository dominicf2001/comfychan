live/templ:
	templ generate --watch --proxy="http://localhost:8080" --cmd="go run ./cmd" --open-browser=false -v

live/sync_assets:
	go run github.com/air-verse/air@v1.61.7 \
	--build.cmd "templ generate --notify-proxy" \
	--build.bin "true" \
	--build.delay "100" \
	--build.exclude_dir "" \
	--build.include_dir "web/static" \
	--build.include_ext "js,css"

live:
	make -j2 live/templ live/sync_assets
