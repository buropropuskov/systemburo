# Dev stage
FROM node:20-alpine AS dev
WORKDIR /app
# Staging VPS периодически теряет connectivity к registry.npmjs.org. Ставим
# агрессивный retry (10 попыток, 20s-2m exponential). Перед npm ci прогреваем
# DNS через explicit lookup чтобы не залипнуть на первом таймауте.
RUN npm config set fetch-retries 10 && \
    npm config set fetch-retry-mintimeout 20000 && \
    npm config set fetch-retry-maxtimeout 120000 && \
    npm config set fetch-timeout 600000
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
EXPOSE 8081
CMD ["npm", "run", "dev", "--", "--host"]

# Builder stage
FROM node:20-alpine AS builder
WORKDIR /app
RUN npm config set fetch-retries 10 && \
    npm config set fetch-retry-mintimeout 20000 && \
    npm config set fetch-retry-maxtimeout 120000 && \
    npm config set fetch-timeout 600000
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
