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
RUN npm ci
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
RUN npm ci
COPY . .
RUN npm run build

# Production stage
FROM nginx:1.25-alpine AS production
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
