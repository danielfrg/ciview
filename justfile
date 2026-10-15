set dotenv-load := true

api:
    go run ./cmd/ciview

frontend:
    pnpm --dir frontend dev

check:
    pnpm --dir frontend install --frozen-lockfile
    pnpm --dir frontend test
    pnpm --dir frontend check
    go test -race ./...
    go vet ./...

build:
    pnpm --dir frontend install --frozen-lockfile
    pnpm --dir frontend test
    pnpm --dir frontend check
    pnpm --dir frontend build
    test -s frontend/dist/index.html
    rm -rf internal/web/dist
    mkdir -p internal/web/dist
    touch internal/web/dist/.keep
    cp -R frontend/dist/. internal/web/dist/
    test -s internal/web/dist/index.html
    go test ./...
    go vet ./...
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ciview ./cmd/ciview
