# Инструкция по развёртыванию фронтенда Dockee

Документ описывает процесс развёртывания клиентской части сервиса Dockee — одностраничного приложения (SPA) на Vue 3. Бэкенд и база данных разворачиваются отдельно и в этом документе не рассматриваются.

## 1. Требования к окружению

### 1.1 Сервер (production)

| Параметр | Минимум | Рекомендуется |
|---|---|---|
| CPU | 1 ядро | 2 ядра |
| RAM | 512 МБ | 1 ГБ |
| Диск | 1 ГБ SSD | 5 ГБ SSD |
| ОС | Ubuntu 22.04 LTS / Debian 12 | Ubuntu 24.04 LTS |
| Сеть | публичный IP, 80 и 443 порты | + CDN |

Фронтенд — статические файлы, серверная нагрузка минимальна. Основную нагрузку берёт на себя веб-сервер (nginx) и CDN.

### 1.2 Программное обеспечение

- **Node.js**: 20.x LTS (для сборки). На production-сервере Node.js не требуется — только готовая сборка.
- **npm**: 10.x (идёт в комплекте с Node.js).
- **Git**: 2.40+.
- **nginx**: 1.24+.
- **certbot** (для Let's Encrypt): последняя версия.

### 1.3 Домены и сертификаты

- Основной домен фронтенда: `app.dockee.ru`.
- API-домен бэкенда (упоминается в переменных окружения): `api.dockee.ru`.
- TLS-сертификат Let's Encrypt, обновляется certbot автоматически.

## 2. Переменные окружения

Переменные задаются в файле `.env.production` в корне репозитория фронтенда **до сборки**. Vite вшивает их значения в bundle на этапе `npm run build`.

```ini
# API-адрес бэкенда
VITE_API_BASE_URL=https://api.dockee.ru/v1

# Ключ публичного клиента платёжного шлюза
VITE_PAYMENT_PUBLIC_KEY=pk_live_xxxxxxxxxxxxxx

# Идентификатор проекта для системы мониторинга ошибок
VITE_SENTRY_DSN=https://<hash>@o<org>.ingest.sentry.io/<project>

# Включение аналитики (только в production)
VITE_ANALYTICS_ENABLED=true

# Имя среды (влияет на баннеры, логирование)
VITE_ENV=production

# Версия сборки (подставляется автоматически из git tag в CI)
VITE_APP_VERSION=1.0.0
```

Файл `.env.production` **не коммитится в репозиторий**. Для локальной разработки используется `.env.development` с адресом `http://localhost:8080/v1`.

## 3. Сборка

### 3.1 Локальная разработка

```bash
git clone git@github.com:dockee-project/frontend.git
cd frontend
npm ci
cp .env.example .env.development
# заполнить .env.development значениями для dev-окружения
npm run dev
```

Dev-сервер Vite запускается на `http://localhost:5173` с горячей перезагрузкой и прокси-настройкой на бэкенд.

### 3.2 Production-сборка

```bash
git checkout main
git pull origin main
npm ci
npm run build
```

Результат — папка `dist/` со статикой: `index.html`, `assets/*.js`, `assets/*.css`, `assets/*.woff2`, иконки. Размер типовой сборки — около 1,2 МБ (450 КБ gzip).

### 3.3 Проверка сборки

```bash
npm run preview
```

Запускает локальный HTTP-сервер на `http://localhost:4173`, имитирующий production-отдачу статики. Используется для тестов перед деплоем.

### 3.4 Линтинг и тесты

```bash
npm run lint          # ESLint + Prettier
npm run test          # Vitest: юнит-тесты
npm run test:e2e      # Playwright: сквозные сценарии
```

В CI эти команды запускаются обязательно — сборка блокируется при ошибках.

## 4. Развёртывание на production

### 4.1 Первичная подготовка сервера

```bash
# Установка nginx
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx

# Создание директории для статики
sudo mkdir -p /var/www/dockee-frontend
sudo chown -R $USER:www-data /var/www/dockee-frontend
sudo chmod -R 750 /var/www/dockee-frontend
```

### 4.2 Конфигурация nginx

Файл `/etc/nginx/sites-available/dockee-frontend.conf`:

```nginx
server {
    listen 80;
    server_name app.dockee.ru;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name app.dockee.ru;

    root /var/www/dockee-frontend/current;
    index index.html;

    ssl_certificate     /etc/letsencrypt/live/app.dockee.ru/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/app.dockee.ru/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;

    # Безопасность
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'nonce-$request_id'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' https://api.dockee.ru; font-src 'self' data:; frame-ancestors 'none';" always;

    # Кэширование статики
    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, immutable";
        access_log off;
    }

    location = /index.html {
        add_header Cache-Control "no-store, no-cache, must-revalidate";
    }

    # SPA fallback: все маршруты ведут на index.html
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Gzip / Brotli
    gzip on;
    gzip_types text/css application/javascript application/json image/svg+xml;
    gzip_min_length 1024;
    gzip_vary on;
}
```

Активация:

```bash
sudo ln -s /etc/nginx/sites-available/dockee-frontend.conf /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### 4.3 Выпуск TLS-сертификата

```bash
sudo certbot --nginx -d app.dockee.ru --email admin@dockee.ru --agree-tos --redirect
```

Certbot автоматически правит конфиг nginx и ставит задачу автообновления в cron.

### 4.4 Первый деплой

Со сборочной машины:

```bash
# 1. Собрать
npm run build

# 2. Загрузить на сервер
rsync -avz --delete ./dist/ deploy@app.dockee.ru:/var/www/dockee-frontend/releases/v1.0.0/

# 3. На сервере переключить симлинк
ssh deploy@app.dockee.ru
cd /var/www/dockee-frontend
ln -sfn releases/v1.0.0 current
```

Такой подход (releases + симлинк current) позволяет мгновенно откатиться: достаточно перевесить симлинк на предыдущий релиз.

## 5. Деплой через Docker (альтернатива)

### 5.1 Dockerfile (multi-stage)

```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ARG VITE_API_BASE_URL
ARG VITE_PAYMENT_PUBLIC_KEY
ARG VITE_SENTRY_DSN
ARG VITE_APP_VERSION
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL \
    VITE_PAYMENT_PUBLIC_KEY=$VITE_PAYMENT_PUBLIC_KEY \
    VITE_SENTRY_DSN=$VITE_SENTRY_DSN \
    VITE_APP_VERSION=$VITE_APP_VERSION
RUN npm run build

FROM nginx:1.27-alpine AS production
COPY --from=builder /app/dist /usr/share/nginx/html
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s CMD wget -q -O- http://localhost/ || exit 1
```

### 5.2 Сборка и запуск

```bash
docker build \
  --build-arg VITE_API_BASE_URL=https://api.dockee.ru/v1 \
  --build-arg VITE_APP_VERSION=1.0.0 \
  -t dockee-frontend:1.0.0 .

docker run -d \
  --name dockee-frontend \
  --restart unless-stopped \
  -p 8080:80 \
  dockee-frontend:1.0.0
```

В production перед контейнером ставится reverse-proxy nginx с TLS — описан в пункте 4.2.

## 6. CI/CD (GitHub Actions)

Файл `.github/workflows/deploy.yml`:

```yaml
name: Deploy frontend

on:
  push:
    tags:
      - 'v*'

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
      - run: npm ci
      - run: npm run lint
      - run: npm run test
      - name: Build
        env:
          VITE_API_BASE_URL: ${{ secrets.VITE_API_BASE_URL }}
          VITE_PAYMENT_PUBLIC_KEY: ${{ secrets.VITE_PAYMENT_PUBLIC_KEY }}
          VITE_SENTRY_DSN: ${{ secrets.VITE_SENTRY_DSN }}
          VITE_APP_VERSION: ${{ github.ref_name }}
        run: npm run build
      - name: Deploy via rsync
        uses: burnett01/rsync-deployments@7.0.1
        with:
          switches: -avz --delete
          path: dist/
          remote_path: /var/www/dockee-frontend/releases/${{ github.ref_name }}/
          remote_host: app.dockee.ru
          remote_user: deploy
          remote_key: ${{ secrets.SSH_PRIVATE_KEY }}
      - name: Switch symlink
        uses: appleboy/ssh-action@v1.0.3
        with:
          host: app.dockee.ru
          username: deploy
          key: ${{ secrets.SSH_PRIVATE_KEY }}
          script: |
            cd /var/www/dockee-frontend
            ln -sfn releases/${{ github.ref_name }} current
```

Деплой запускается автоматически при пуше git-тега вида `v1.2.3`.

## 7. Проверка после деплоя

1. Открыть `https://app.dockee.ru` в режиме инкогнито.
2. В DevTools на вкладке «Network» убедиться, что все ресурсы отдаются с кодом 200 и сжаты (gzip/br).
3. Проверить версию в футере страницы (совпадает с задеплоенным тегом).
4. Пройти сценарий входа и загрузки тестового документа.
5. В консоли не должно быть ошибок уровня error.
6. Запросить `https://app.dockee.ru/nonexistent/path` — должен отдать `index.html` со статусом 200 (SPA fallback).
7. Lighthouse audit: все четыре категории ≥ 90 баллов.

## 8. Откат релиза

```bash
ssh deploy@app.dockee.ru
cd /var/www/dockee-frontend
ls releases/                             # посмотреть доступные релизы
ln -sfn releases/v1.0.0 current          # переключить на нужный
sudo systemctl reload nginx              # не требуется для симлинка, но обновит кэш
```

Откат занимает секунды, так как файлы уже на сервере. Cache-busting обеспечивает `index.html` c `Cache-Control: no-store` — пользователи мгновенно получат предыдущую версию bundle.
