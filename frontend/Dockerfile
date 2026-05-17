# syntax=docker/dockerfile:1.4
# syntax-директива требуется для поддержки --mount=type=cache. Без неё cache
# mount работает в no-op режиме и не ускоряет сборку.

# Dev stage
FROM node:20-alpine AS dev
WORKDIR /app
# Staging Jino VPS (RU) теряет connectivity к registry.npmjs.org (Cloudflare).
# Переключаем на npmmirror.com (Китай, доступен из РФ, честное зеркало - integrity
# hashes из package-lock.json валидны). Retry оставляем агрессивный через ENV,
# который npm точно подхватывает (в отличие от 'npm config set' в RUN-layer).
ENV NPM_CONFIG_REGISTRY=https://registry.npmmirror.com \
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
FROM node:20-alpine AS builder
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
