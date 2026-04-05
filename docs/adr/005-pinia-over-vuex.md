# ADR-005: Pinia вместо Vuex

**Статус:** Принято

## Контекст

Нужен state management для Vue 3. Vuex 4 — официальный, но в maintenance mode. Pinia — рекомендованная замена от автора Vue.

## Решение

Pinia. Три store: auth (токены), permissions (права), ui (toast, sidebar).

## Последствия

**Плюсы:**
- Нет mutations — action напрямую меняет state (вдвое меньше кода)
- Каждый store — отдельный `defineStore`, импортируется напрямую (нет modules/namespaced)
- TypeScript-ready (если будет миграция)
- 1KB gzip vs 3KB Vuex
- auth.js: 51 строка — state + getters + actions. На Vuex было бы ~80

**Минусы:**
- Нет Vuex devtools time-travel (Pinia devtools есть, но менее зрелые)
- Меньше StackOverflow-ответов (быстро меняется — Pinia становится стандартом)
