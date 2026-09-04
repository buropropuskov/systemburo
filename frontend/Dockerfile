# syntax=docker/dockerfile:1.4
# syntax-директива требуется для поддержки --mount=type=cache. Без неё cache
# mount работает в no-op режиме и не ускоряет сборку.

# Dev stage
FROM node:22-alpine AS dev
WORKDIR /app
# Возвращаемся на registry.npmjs.org (оригинал). npmmirror.com (Alibaba/Китай)
# работал из РФ, но через РФ→Китай latency высокая - npm ci на слабом VPS Jino
# залипал на 25+ минут даже после warning'ов про deprecated пакеты. Из реальных
# измерений: GET /vue с Jino → npmmirror ~5-8s/пакет, → npmjs ~1-2s/пакет.
# Прежнее обоснование ("теряет connectivity к npmjs Cloudflare") - временная
# проблема Cloudflare октября 2025, давно исправилась.
# Retry агрессивный сохраняем на случай транзиентных сбоев.
ENV NPM_CONFIG_REGISTRY=https://registry.npmjs.org \
    NPM_CONFIG_FETCH_RETRIES=10 \
    NPM_CONFIG_FETCH_RETRY_MINTIMEOUT=30000 \
    NPM_CONFIG_FETCH_RETRY_MAXTIMEOUT=300000 \
    NPM_CONFIG_FETCH_TIMEOUT=900000
COPY package.json package-lock.json ./
# Cache mount для /root/.npm: BuildKit держит npm cache между сборками
# независимо от --no-cache на образе (image-layer cache != cache mount).
# Второй и далее деплой переиспользует скачанные тарболлы → npm ci делает
# только linking, не network. С Jino → npmmirror это даёт x5-x10 ускорения.
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    npm ci
COPY . .
EXPOSE 8081
CMD ["npm", "run", "dev", "--", "--host"]

# Builder stage
FROM node:22-alpine AS builder
WORKDIR /app
ENV NPM_CONFIG_REGISTRY=https://registry.npmmirror.com \
    NPM_CONFIG_FETCH_RETRIES=10 \
    NPM_CONFIG_FETCH_RETRY_MINTIMEOUT=30000 \
    NPM_CONFIG_FETCH_RETRY_MAXTIMEOUT=300000 \
    NPM_CONFIG_FETCH_TIMEOUT=900000
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    npm ci
COPY . .
# Cache mount для vite cache (esbuild transforms, dependency pre-bundle).
# Второй и далее билд переиспользует пред-собранные зависимости → vite build
# отрабатывает за десятки секунд вместо 2-5 минут на слабом CPU.
RUN --mount=type=cache,target=/app/node_modules/.vite,sharing=locked \
    npm run build

# Production stage
FROM nginx:1.25-alpine AS production
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
