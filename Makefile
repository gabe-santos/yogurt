.PHONY: all build frontend backend test test-api test-e2e check clean run

BINARY := bin/reader

all: check

# One binary carrying the API and the compiled SPA.
build: frontend backend

frontend:
	pnpm --dir web install --frozen-lockfile
	pnpm --dir web build

backend:
	go build -o $(BINARY) ./cmd/reader

run: build
	./$(BINARY)

# Seams 1 and 2: the HTTP API and the pure pull policy.
test-api:
	go test ./...

# Seam 3: the browser, against the real binary.
test-e2e: build
	pnpm --dir web exec playwright test

test: test-api test-e2e

check: build
	go vet ./...
	pnpm --dir web run check
	$(MAKE) test

clean:
	rm -rf bin internal/webui/dist/spa web/build web/.svelte-kit
