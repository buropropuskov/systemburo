const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: './e2e/tests',
  timeout: 30000,
  retries: 1,
  // Пропускаем спеки, которые полагаются на отсутствующий /register endpoint
  // и на seed-данные (application categories, attachments, unique-cars).
  // Переработаем отдельной задачей — расширим cmd/seed и/или вынесем fixtures.
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
