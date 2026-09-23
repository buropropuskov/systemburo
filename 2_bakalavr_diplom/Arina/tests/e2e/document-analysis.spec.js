// Сквозной сценарий «Загрузка документа и проверка анализа».
//
// Полный пользовательский путь: вход → загрузка PDF → дождаться результатов →
// проверить подсветку рисков → скачать отчёт.
//
// Стек: Playwright Test. Браузеры: Chromium, Firefox (WebKit отключён
// из-за нехватки зависимостей на Ubuntu CI-сервере — см. playwright.config.js).
//
//возможны изменения// — часть маршрутов и селекторов относится к компонентам
// AnalysisPage.vue, AnalysisResult.vue и AiAssistant.vue, которые на момент
// написания тестов находятся в процессе разработки.

import { test, expect } from '@playwright/test'
import path from 'node:path'

const BASE = process.env.DOKKEE_URL || 'http://localhost:8080'
const FIX  = path.join(__dirname, 'fixtures')

test.describe('Анализ документа', () => {
  test.beforeEach(async ({ page }) => {
    // Быстрый логин через API, cookie/токен подхватывает фронтенд.
    const token = await getTestUserToken(page)
    await page.addInitScript((t) => window.localStorage.setItem('auth_token', t), token)
  })

  test('PDF-документ успешно загружается и показывает риски', async ({ page }) => {
    await page.goto(`${BASE}/service`)

    // Загружаем файл через видимое поле input (type=file, multiple).
    const fileChooser = page.locator('input[type="file"]')
    await fileChooser.setInputFiles(path.join(FIX, 'sample-contract.pdf'))

    // Список файлов: имя должно появиться
    await expect(page.getByText('sample-contract.pdf')).toBeVisible()

    // Согласие на обработку ПД + запуск
    await page.locator('.agreement__checkbox').check()
    await page.getByRole('button', { name: /запустить проверку и анализ/i }).click()

    // Ожидание завершения анализа — индикатор обработки должен исчезнуть
    await expect(page.locator('.upload__gear')).toBeHidden({ timeout: 60_000 })

    // Проверяем наличие хотя бы одного риска в правой панели
    await expect(page.locator('.risk-card').first()).toBeVisible()

    // Подсветка в тексте документа должна появиться
    const highlights = page.locator('mark[data-risk]')
    await expect(highlights.first()).toBeVisible()
  })

  test('попытка загрузить PNG отклоняется с сообщением', async ({ page }) => {
    await page.goto(`${BASE}/service`)

    const fileChooser = page.locator('input[type="file"]')
    await fileChooser.setInputFiles(path.join(FIX, 'photo.png'))

    // В списке ничего не должно появиться (расширение отфильтровано).
    await expect(page.getByText('photo.png')).toHaveCount(0)
    await expect(page.getByText(/загрузите файлы/i)).toBeVisible()
  })

  test('экспорт отчёта PDF после анализа', async ({ page }) => {
    await page.goto(`${BASE}/service`)
    await page.locator('input[type="file"]')
      .setInputFiles(path.join(FIX, 'sample-contract.pdf'))
    await page.locator('.agreement__checkbox').check()
    await page.getByRole('button', { name: /запустить/i }).click()
    await expect(page.locator('.risk-card').first()).toBeVisible({ timeout: 60_000 })

    const [download] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: /скачать отчёт/i }).click(),
    ])
    expect(download.suggestedFilename()).toMatch(/\.pdf$/)
  })
})

// Получает JWT через API. //возможны изменения// — контракт /auth/sign-in пока
// возвращает { token }, при смене схемы обновить функцию.
async function getTestUserToken(page) {
  const res = await page.request.post(`${process.env.API_URL || 'http://localhost:8000'}/auth/sign-in`, {
    data: { email: process.env.TEST_USER_EMAIL, password: process.env.TEST_USER_PASSWORD },
  })
  const body = await res.json()
  return body.token
}
