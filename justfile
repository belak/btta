default:
    @just --list

# Start the dev server with hot-reload (Go + frontend Vite + codegen)
dev: generate
    mprocs --names "server,generate,frontend" \
        "watchexec -r -d 1000ms -e go,css --ignore '**/*_templ.go' --ignore '**/*.sql.go' -- go run ./cmd/btta serve --log-format pretty" \
        "watchexec -e sql,templ --no-vcs-ignore -- just generate" \
        "cd frontend && pnpm run dev"

# Build the frontend (outputs to internal/http/frontend/dist/)
build-frontend:
    cd frontend && pnpm run build

# Run all tests
test *args="": generate
    go test ./... {{ args }}

# Format all code
fmt *args="":
    treefmt {{ args }}

# Generate code (sqlc, templ)
generate:
    go generate ./...
