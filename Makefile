.PHONY: dev build clean vet test up down

# ── Development ────────────────────────────────────────────────────────

dev: up
	go run ./cmd/server

build:
	go build -o bin/forgeos-server ./cmd/server

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf bin/

# ── Docker infrastructure ──────────────────────────────────────────────

up:
	docker compose up -d

down:
	docker compose down

restart: down up

# ── Quick smoke test (after `make dev`) ───────────────────────────────
smoke:
	@echo "=== Health check ==="
	curl -sf http://localhost:8081/api/v1/system/health | python -m json.tool
	@echo ""
	@echo "=== Register ==="
	curl -sf -X POST http://localhost:8081/api/v1/auth/register \
		-H "Content-Type: application/json" \
		-d '{"email":"test@forgeos.local","password":"testpass123","name":"Test User"}' | python -m json.tool
