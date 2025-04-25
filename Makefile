live/templ:
	templ generate --watch --proxy="http://localhost:8080" --cmd="go run ./web" --open-browser=false -v

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

db/seed:
	sqlite3 ./internal/database/comfychan.db < ./internal/database/seed.sql

db/seed/f:
	rm ./internal/database/comfychan.db && sqlite3 ./internal/database/comfychan.db < ./internal/database/seed.sql

build: 
	go build -o ./out/comfychan ./web

deploy:
	sudo mkdir -p /var/lib/comfychan/web/static
	sudo mkdir -p /var/lib/comfychan/internal/database
	sudo cp ./out/comfychan /srv/comfychan/
	sudo cp -r web/static/* /var/lib/comfychan/web/static/
	sudo chown root:root /srv/comfychan/comfychan

db/deploy:
	sudo mkdir -p /var/lib/comfychan/internal/database
	sudo sqlite3 /var/lib/comfychan/internal/database/comfychan.db < ./internal/database/seed.sql
