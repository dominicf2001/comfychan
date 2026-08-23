# comfychan

A small imageboard server written in Go, using [chi](https://github.com/go-chi/chi) for routing, [templ](https://templ.guide) for HTML templating, [htmx](https://htmx.org) for interactivity, and SQLite for storage.

## Requirements

- Go 1.25+
- [templ CLI](https://templ.guide/quick-start/installation) (`go install github.com/a-h/templ/cmd/templ@latest`), matching the version in `go.mod`
- `sqlite3` (for seeding the database)

## Development

```
make live
```

Starts the app with live reload: `templ` watches and regenerates `.templ` files, and static assets (`web/static`) trigger a browser refresh via proxy on `http://localhost:8080`.

Seed the database:

```
make db/seed      # seed
make db/seed/f    # wipe and reseed
```

## Building

```
make build
```

Generates templates and builds a binary at `./out/comfychan`.

## Configuration

- `COMFYCHAN_DATA_DIR` — base directory for runtime data (SQLite database, uploaded media, static assets). Defaults to the current directory if unset.

## Deployment

`./deploy.sh` builds the binary and ships it to a self-hosted server running comfychan as a systemd service (see script for target host/paths).
