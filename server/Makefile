.PHONY: generate build run clean dev test

generate: ## Generate templ files
	templ generate

build: generate ## Build binary
	CGO_ENABLED=0 go build -o xpbx ./cmd/xpbx

run: generate ## Run locally
	go run ./cmd/xpbx

clean: ## Clean build artifacts
	rm -f xpbx

test: ## Run tests
	go test -race -count=1 ./...

dev: generate ## Run with live reload (requires air)
	air -c .air.toml
