.PHONY: up down build test logs restart lint swagger bash db-shell frontend-dev prod-build init init-staging init-production seed seed-demo staging-seed staging-seed-demo deploy-seed deploy-seed-demo staging-build staging-up staging-down staging-logs deploy-build deploy-up deploy-down deploy-logs security maintenance-off staging-maintenance-off deploy-maintenance-off cleanup staging-cleanup deploy-cleanup storage staging-storage deploy-storage archive staging-archive deploy-archive entity staging-entity deploy-entity fake staging-fake vapid staging-vapid deploy-vapid backup staging-backup deploy-backup backup-status staging-backup-status deploy-backup-status backup-verify staging-backup-verify deploy-backup-verify deploy-restore restore staging-restore

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

swagger:
	docker compose exec go-backend swag init -g cmd/server/main.go -o docs

bash:
	docker compose exec go-backend sh

db-shell:
	docker compose exec db psql -U postgres -d auto_registry

frontend-dev:
	docker compose up -d frontend

prod-build:
	docker build --target production -t systemburo:latest .

# Создать/обновить тестового админа (buropropuskov / admin123)
# Кастомный пароль: make seed PASS=mypass (уходит в сидер как -password)
seed:
	docker compose exec go-backend go run ./cmd/seed $(if $(PASS),-password $(PASS))

staging-seed:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml exec backend ./seed $(if $(PASS),-password $(PASS))

deploy-seed:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml exec backend ./seed $(if $(PASS),-password $(PASS))

# Демо-данные для UI-сценариев (объявления, новости, заявки с вложениями, cars_history).
# Не запускать на production без явной необходимости.
seed-demo:
	docker compose exec -e SEED_DEMO=true go-backend go run ./cmd/seed $(if $(PASS),-password $(PASS))

staging-seed-demo:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml exec -e SEED_DEMO=true backend ./seed $(if $(PASS),-password $(PASS))

deploy-seed-demo:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml exec -e SEED_DEMO=true backend ./seed $(if $(PASS),-password $(PASS))

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

# Обзор занятого места: крупнейшие таблицы и что из них подлежит очистке.
# Только читает. Справка: make storage ARGS=-help
storage:
	docker compose exec go-backend go run ./cmd/server storage $(ARGS)

staging-storage:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml exec backend ./server storage $(ARGS)

deploy-storage:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml exec backend ./server storage $(ARGS)

# Настройка файлового архива бланков: раскладка каталогов, пороги места, заморозка.
# Без ARGS печатает справку. Примеры: make archive ARGS="show"
#                                     make deploy-archive ARGS="set -freeze 60 -min-free 4G"
archive:
	docker compose exec go-backend go run ./cmd/server archive $(ARGS)

staging-archive:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml exec backend ./server archive $(ARGS)

deploy-archive:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml exec backend ./server archive $(ARGS)

# Работа с данными по идентификатору сущности: снять пакет, проверить его и развернуть на
# другом стенде. Без ARGS печатает справку. Примеры: make entity ARGS="export -type=organization -id=42"
#                                                     make deploy-entity ARGS="export -type=organization -id=42 -apply"
entity:
	docker compose exec go-backend go run ./cmd/server entity $(ARGS)

staging-entity:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml exec backend ./server entity $(ARGS)

deploy-entity:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml exec backend ./server entity $(ARGS)

# Наполнение проверочного стенда вымышленными данными. Без ARGS показывает план и
# ничего не создаёт. Справка: make fake ARGS=-help
# Примеры: make staging-fake ARGS="-mark-stand"
#          make staging-fake ARGS="-profile=large -apply"
# Цели для рабочего сервера намеренно нет: вымышленные данные туда не наливают.
fake:
	docker compose exec go-backend go run ./cmd/server fake $(ARGS)

staging-fake:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml exec backend ./server fake $(ARGS)

# Генерация пары ключей VAPID для Web Push (#974). Не трогает базу - только печатает
# готовые строки для файла параметров (VAPID_PUBLIC_KEY/VAPID_PRIVATE_KEY).
vapid:
	docker compose exec go-backend go run ./cmd/server vapid $(ARGS)

staging-vapid:
	docker compose -f docker-compose.base.yml -f docker-compose.staging.yml exec backend ./server vapid $(ARGS)

deploy-vapid:
	docker compose -f docker-compose.base.yml -f docker-compose.prod.yml exec backend ./server vapid $(ARGS)

# Резервное копирование. Снимает выгрузку базы и архив загруженных файлов,
# Метка в имени файла: make deploy-backup ARGS=pered-obnovleniem
# раскладывает по срокам хранения и чистит устаревшие копии.
# Расписание ставится один раз: sudo ./scripts/backup-install.sh production
backup:
	bash scripts/backup.sh local $(ARGS)

staging-backup:
	bash scripts/backup.sh staging $(ARGS)

deploy-backup:
	bash scripts/backup.sh production $(ARGS)

# Состояние копирования: когда снята последняя копия, сколько их и не устарели ли.
backup-status:
	bash scripts/backup-status.sh local

staging-backup-status:
	bash scripts/backup-status.sh staging

deploy-backup-status:
	bash scripts/backup-status.sh production

# Проверка восстановимости: копия разворачивается во временную базу рядом с рабочей.
backup-verify:
	bash scripts/backup-verify.sh local $(ARGS)

staging-backup-verify:
	bash scripts/backup-verify.sh staging $(ARGS)

deploy-backup-verify:
	bash scripts/backup-verify.sh production $(ARGS)

# Восстановление из копии. Необратимо, требует подтверждения.
# Пример: make deploy-restore ARGS="/var/backups/systemburo/daily/buro-db-2026-07-31-0330.dump.age"
restore:
	bash scripts/restore.sh local $(ARGS)

staging-restore:
	bash scripts/restore.sh staging $(ARGS)

deploy-restore:
	bash scripts/restore.sh production $(ARGS)
