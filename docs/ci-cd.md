# CI/CD контракт

## Ветки и защита

| Ветка | Что значит | Deploy куда | Защита (когда доступно) |
|---|---|---|---|
| `dev` | Рабочая интеграционная, все feature-PR сюда | автоматически на staging | ruleset: нельзя удалить/force-push, PR + `ci-summary` зелёный |
| `staging` | Синхронизируется с `dev` после успешного deploy | — (читается нами, для истории) | ruleset |
| `prod` | Production, default-ветка | автоматически на production (через environment approval) | ruleset + required review 1 |

Feature-ветки (`issue/N`, `fix/...`) — без защиты, автоматически удаляются после merge (`delete_branch_on_merge: true`).

### Текущее ограничение

На 2026-04-21 API `/rulesets` и `/branches/.../protection` возвращают **403 Upgrade required** для этой org. Это означает что защита настраивается либо вручную через `Settings → Branches → Add rule` в UI (если функция открыта), либо через обходной workflow, либо после апгрейда плана. Пока ветка `dev` защищена только аккуратностью — `delete_branch_on_merge` включён на репо-уровне и будет чистить source-branch при merge; **для target-ветки (`dev`, `prod`) это безопасно** — GitHub удаляет только source PR, не target.

Важный момент: **merge PR с target = `dev`/`prod` НЕ удаляет `dev`/`prod`** (удалялась только source: ранее `dev` была source в PR #72 `dev → prod`). Feature-ветки (source) будут удалены как обычно.

Запрет force-push настроен опосредованно через политику репо (можно включить через UI в Rules → New branch ruleset, даже если API недоступен).

## Workflows

- **`ci.yml`** — главный пайплайн на `push`/`pull_request`. `paths-filter` скипает Go при frontend-only PR и наоборот. DAG: vet/lint → test → docker-build → deploy-staging (только `push:dev`) → `ci-summary`. `concurrency: cancel-in-progress` на PR.
- **`deploy-production.yml`** — `push:prod` или `workflow_dispatch` (нужно ввести `deploy`). Environment `production` с required reviewer.
- **`e2e.yml`** — Playwright на PR в `dev`. Поднимает postgres, собирает бэк, seed-юзер `buropropuskov`, фронт на 8081. Пока **не required** gate — стабилизируется.
- **`security.yml`** — govulncheck + npm audit + Trivy (`ignore-unfixed: true`) + `actions/dependency-review-action` на PR. Schedule Mon 06:00.
- **`dependabot.yml`** — weekly PR по gomod / npm / github-actions / docker. Группы: gorm+pgx, vue/vite, playwright.

## Composite actions (`.github/actions/`)

- `setup-go` — setup-go@v5 с cache + go mod download.
- `setup-frontend` — setup-node@v4 + npm ci в `frontend/`.
- `playwright-cache` — cache `~/.cache/ms-playwright` по версии пакета.

## Secrets и environments

`staging` и `production` — GitHub Environments. Хранят `VPS_HOST`, `VPS_PORT`, `VPS_USER`, `VPS_SSH_KEY`, `STAGING_DOMAIN`/`PROD_DOMAIN`. Password SSH удалён, только key. `deployment_branch_policy` ограничивает какие ветки могут деплоить в этот env (dev → staging, prod → production).

## Как выкатить фикс в prod

1. PR в `dev` → merge после зелёного `ci-summary` → auto deploy staging.
2. Ручная проверка на `https://stagingburo.washka17.ru`.
3. PR `dev → prod` → merge после зелёного CI + 1 approve.
4. GitHub отправит event `push:prod`, `deploy-production.yml` запустится, environment `production` попросит approve → deploy.

## Как чинить упавший деплой

- **deploy-staging упал в `ci.yml`**: SSH на VPS, `cd /opt/systemburo && docker compose -f docker-compose.base.yml -f docker-compose.staging.yml logs backend --tail=200`. Откатить через `git revert` проблемного коммита → push → переедет.
- **deploy-production упал**: то же самое, но `prod.yml`. Плюс можно откатить PR `dev → prod` (revert merge commit на prod).
- **CI красный после изменения workflow**: Ruleset в `evaluate` mode для отладки, потом обратно в `active`.

## Лимиты GitHub Free

- 2000 минут actions / месяц на org private.
- 10 GB cache per repo, LRU-вытеснение (TTL 7 дней без доступа).
- 20 concurrent jobs (Linux).
- Ruleset + Environments + required status checks — доступны бесплатно для org private.
