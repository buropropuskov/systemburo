# Рабочий процесс

## Ветки

```
dev       — разработка, сюда мержатся фичи
staging   — зеркало dev, обновляется автоматически после деплоя
prod      — релиз, обновляется через PR из staging
```

- `dev` — основная рабочая ветка
- `staging` — не трогать руками, синхронизируется автоматически
- `prod` — защищена, мерж только через PR с approve от @washka20

## Workflow: от задачи до прода

```
1. Берёшь issue (#123)
2. Создаёшь ветку: issue/123
3. Работаешь, коммитишь
4. Создаёшь PR в dev
5. CI проверяет тесты + формат коммитов
6. Мержишь в dev
7. CI на dev зелёный → авто-деплой на staging
8. Проверяешь на staging
9. PR staging → prod → approve → ручной деплой
```

## Ветки для задач

Формат: `issue/НОМЕР`

```bash
git checkout dev
git pull origin dev
git checkout -b issue/123
```

## Коммиты

Формат: `prefix(scope): описание`

**Префиксы:**
- `feat:` — новая функциональность
- `fix:` — исправление бага
- `refactor:` — рефакторинг без изменения поведения
- `docs:` — документация
- `test:` — тесты
- `chore:` — рутина (зависимости, конфиги)
- `style:` — форматирование, без изменения логики
- `build:` — сборка, Docker
- `ci:` — CI/CD
- `perf:` — оптимизация производительности

**Scope** опционален: `feat(auth): добавил JWT` или `fix: исправил валидацию`

**Глагол** в прошедшем времени: `добавил`, `исправил`, `вынес`, `обновил`

**Ссылка на issue** в конце: `fix: исправил валидацию (#123)`

### Примеры

```
feat: добавил фильтр заявок по дате (#45)
fix(auth): исправил обновление refresh-токена (#67)
refactor: вынес хелперы в отдельный пакет
test: добавил тесты для employee service
ci: обновил Node.js 18 → 20
```

### Плохие коммиты (hook отклонит)

```
исправил баг                    ← нет префикса
fix: ок                         ← слишком короткое описание
Update main.go                  ← нет префикса, английский
FIX: что-то                     ← префикс с большой буквы
```

## Pull Requests

### PR в dev (из issue/*)

```bash
gh pr create --base dev --title "feat: описание (#123)" --body "Closes #123"
```

- CI должен пройти (тесты + lint коммитов)
- После мержа issue закроется автоматически (если есть `Closes #123`)

### PR в prod (из staging)

```bash
gh pr create --base prod --head staging --title "release: описание"
```

- Обязательный approve от @washka20
- CI должен пройти
- После мержа — ручной деплой через Actions → Deploy → Run workflow → production

## Issues

### Создание

GitHub → Issues → New issue → выбрать шаблон (Bug / Feature)

### Labels

| Label | Когда использовать |
|-------|--------------------|
| `bug` | Что-то сломано |
| `feature` | Новая функциональность |
| `enhancement` | Улучшение существующего |
| `refactor` | Рефакторинг |
| `frontend` | Касается фронтенда |
| `backend` | Касается бэкенда |
| `ci` | CI/CD, деплой |
| `priority:high` | Срочно |
| `priority:low` | Не горит |

### Project Board

[Доска задач](https://github.com/orgs/buropropuskov/projects/1)

Колонки: Backlog → In Progress → Review → Done

## CI/CD

| Событие | Что происходит |
|---------|---------------|
| PR в dev | CI: тесты Go + frontend lint + commit lint |
| Push/мерж в dev | CI → зелёный → авто-деплой staging → sync staging ветки |
| PR в prod | CI + security checks |
| Мерж в prod | Ручной деплой через Actions |

## Локальная настройка

```bash
# Клонировать
git clone git@github.com:buropropuskov/systemburo.git
cd systemburo

# Настроить git hooks (проверка формата коммитов)
make init

# Запустить dev-окружение
cp .env.example .env  # отредактировать
make up
```
