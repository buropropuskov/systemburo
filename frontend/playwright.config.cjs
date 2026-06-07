const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: './e2e/tests',
  // Прогрев холодного vite dev-сервера до старта тестов (см. e2e/global-setup.cjs) -
  // иначе первые параллельные навигации шарда проигрывают 5s-таймауту toHaveURL.
  globalSetup: require.resolve('./e2e/global-setup.cjs'),
  timeout: 30000,
  retries: 1,
  // Пропущенные спеки - полагаются на отсутствующий /register endpoint
  // или специфические seed-данные (application categories, attachments).
  // Переработаем расширением cmd/seed и/или вынесем fixtures.
  testIgnore: [
    '**/auth.spec.{js,cjs}',
    '**/application-lifecycle.spec.{js,cjs}',
    '**/application-center.spec.{js,cjs}',
    '**/application-create.spec.{js,cjs}',
  ],
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:8081',
    headless: true,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
    ...(process.env.E2E_HTTP_USER && {
      httpCredentials: {
        username: process.env.E2E_HTTP_USER,
        password: process.env.E2E_HTTP_PASSWORD || '',
      },
    }),
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
});
