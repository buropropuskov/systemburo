# ADR-006: Options API вместо Composition API

**Статус:** Принято

## Контекст

Vue 3 поддерживает два стиля: Options API (data/methods/computed/lifecycle) и Composition API (`<script setup>`, composables). 50+ компонентов в проекте.

## Решение

Options API для всех компонентов. Composition API используется только в composables (useFormValidation, useToast, useDropdownState, useEntitySelection).

## Последствия

**Плюсы:**
- Консистентность — все 50+ компонентов в одном стиле
- Низкий порог входа — Options API интуитивнее для новых разработчиков
- Чёткая визуальная структура: props → data → computed → methods → lifecycle

**Минусы:**
- Хуже масштабируется — в компонентах 1000+ строк логика разбросана по секциям (data в одном месте, methods в другом, watch в третьем)
- Composition API лучше для переиспользования логики (composables vs mixins)
- Тренд экосистемы — большинство новых библиотек и примеров на Composition API

**Рекомендация:** Новые компоненты можно писать на Composition API (`<script setup>`). Старые переводить при рефакторинге, не специально.
