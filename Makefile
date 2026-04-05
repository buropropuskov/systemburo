.PHONY: up down build test logs restart lint bash db-shell frontend-dev prod-build init init-staging init-production staging-build staging-up staging-down staging-logs deploy-build deploy-up deploy-down deploy-logs security

up:
	docker compose up -d

down:
	docker compose down

build:
	docker compose build

test:
	docker compose exec go-backend go test ./... -v -count=1

logs:
	docker compose logs -f

restart:
	docker compose restart

lint:
	docker compose exec go-backend go vet ./...

bash:
	docker compose exec go-backend sh

db-shell:
	docker compose exec db psql -U postgres -d auto_registry

frontend-dev:
	docker compose up -d frontend

prod-build:
	docker build --target production -t systemburo:latest .

init:
	git config core.hooksPath .githooks
	@echo "Git hooks настроены на .githooks/"

init-staging:
	bash scripts/init-env.sh staging $(DOMAIN)

init-production:
	bash scripts/init-env.sh production $(DOMAIN)

staging-build:
	docker compose -f docker-compose.staging.yml build

staging-up:
	docker compose -f docker-compose.staging.yml up -d

staging-down:
	docker compose -f docker-compose.staging.yml down

staging-logs:
	docker compose -f docker-compose.staging.yml logs -f

deploy-build:
	docker compose -f docker-compose.prod.yml build

deploy-up:
	docker compose -f docker-compose.prod.yml up -d

deploy-down:
	docker compose -f docker-compose.prod.yml down

deploy-logs:
	docker compose -f docker-compose.prod.yml logs -f

security:
	go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...
	cd frontend && npm audit --audit-level=high || true
