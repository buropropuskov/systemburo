.PHONY: up down build test logs restart lint bash db-shell frontend-dev prod-build init init-staging init-production seed seed-demo staging-seed staging-seed-demo deploy-seed deploy-seed-demo staging-build staging-up staging-down staging-logs deploy-build deploy-up deploy-down deploy-logs security maintenance-off staging-maintenance-off deploy-maintenance-off cleanup staging-cleanup deploy-cleanup

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

# Создать/обновить тестового админа (buropropuskov / admin123)
# Кастомный пароль: make seed PASS=mypass
seed:
	docker compose exec go-backend go run ./cmd/seed $(PASS)

staging-seed:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml exec backend ./seed $(PASS)

deploy-seed:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml exec backend ./seed $(PASS)

# Демо-данные для UI-сценариев (объявления, новости, заявки с вложениями, cars_history).
# Не запускать на production без явной необходимости.
seed-demo:
	docker compose exec -e SEED_DEMO=true go-backend go run ./cmd/seed $(PASS)

staging-seed-demo:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml exec -e SEED_DEMO=true backend ./seed $(PASS)

deploy-seed-demo:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml exec -e SEED_DEMO=true backend ./seed $(PASS)

init:
	git config core.hooksPath .githooks
	@echo "Git hooks настроены на .githooks/"

init-staging:
	bash scripts/init-env.sh staging $(DOMAIN)

init-production:
	bash scripts/init-env.sh production $(DOMAIN)

staging-build:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml build

staging-up:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml up -d

staging-down:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml down

staging-logs:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml logs -f

deploy-build:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml build

deploy-up:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d

deploy-down:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml down

deploy-logs:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml logs -f

security:
	go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...
	cd frontend && npm audit --audit-level=high || true

# Аварийное снятие режима технических работ мимо интерфейса - когда супер-админ
# не может войти. Режим отпускает пользователей в течение 10 секунд (TTL кэша),
# перезапуск сервера не нужен.
maintenance-off:
	bash scripts/maintenance-off.sh local

staging-maintenance-off:
	bash scripts/maintenance-off.sh staging

deploy-maintenance-off:
	bash scripts/maintenance-off.sh production

# Очистка накопленных данных. Без ARGS показывает, сколько записей попадёт под
# удаление, и ничего не удаляет. Справка: make cleanup ARGS=-help
# Примеры: make cleanup ARGS="-apply"
#          make deploy-cleanup ARGS="-targets=audit -older-than=36m"
cleanup:
	docker compose exec go-backend go run ./cmd/server cleanup $(ARGS)

staging-cleanup:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml exec backend ./server cleanup $(ARGS)

deploy-cleanup:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml exec backend ./server cleanup $(ARGS)
