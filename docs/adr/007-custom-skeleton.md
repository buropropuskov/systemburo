# ADR-007: Кастомные skeleton-компоненты

**Статус:** Принято

## Контекст

Страницы показывали пустой экран или CSS-спиннер при загрузке данных. Нужны skeleton-заглушки. Варианты: vue-content-loader (SVG), vue-skeleton-loader (npm), кастомные компоненты.

## Решение

5 кастомных компонентов: SkeletonLine, SkeletonBlock, SkeletonTable, SkeletonCard, SkeletonTransition. CSS shimmer-анимация через gradient + `::after`.

## Последствия

**Плюсы:**
- Нет внешних зависимостей (~200 строк CSS)
- Интеграция с design tokens (--color-skeleton, --color-skeleton-shine)
- SkeletonTransition: delay 200ms (не мелькает на быстрой сети), minDuration 400ms (не исчезает мгновенно)
- Единая анимация cubic-bezier(0.4, 0, 0.2, 1) — совпадает с NavMenu

**Минусы:**
- Нет content-aware скелетов (SVG по форме контента, как vue-content-loader)
- Поддержка своими силами

Интегрировано в 8 страниц.
