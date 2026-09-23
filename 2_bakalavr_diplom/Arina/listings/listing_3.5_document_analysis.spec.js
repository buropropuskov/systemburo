// Листинг 3.5 — Сквозной сценарий загрузки и анализа документа (Playwright)
// Файл: tests/e2e/document-analysis.spec.js

import { test, expect } from '@playwright/test'
import path from 'node:path'

const BASE = process.env.DOKKEE_URL || 'http://localhost:8080'
const FIX  = path.join(__dirname, 'fixtures')

test.describe('Анализ документа', () => {
  test.beforeEach(async ({ page }) => {
    const token = await getTestUserToken(page)
    await page.addInitScript((t) => window.localStorage.setItem('auth_token', t), token)
  })

  test('PDF-документ успешно загружается и показывает риски', async ({ page }) => {
    await page.goto(`${BASE}/service`)

    await page.locator('input[type="file"]')
      .setInputFiles(path.join(FIX, 'sample-contract.pdf'))
    await expect(page.getByText('sample-contract.pdf')).toBeVisible()

    await page.locator('.agreement__checkbox').check()
    await page.getByRole('button', { name: /запустить проверку и анализ/i }).click()

    // ожидание завершения анализа — индикатор обработки должен исчезнуть
    await expect(page.locator('.upload__gear')).toBeHidden({ timeout: 60_000 })

    // в правой панели должна появиться хотя бы одна карточка риска
    await expect(page.locator('.risk-card').first()).toBeVisible()

    // в тексте документа — хотя бы одна подсветка
    await expect(page.locator('mark[data-risk]').first()).toBeVisible()
  })

  test('попытка загрузить PNG отклоняется с сообщением', async ({ page }) => {
    await page.goto(`${BASE}/service`)
    await page.locator('input[type="file"]')
      .setInputFiles(path.join(FIX, 'photo.png'))

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

async function getTestUserToken(page) {
  const res = await page.request.post(
    `${process.env.API_URL || 'http://localhost:8000'}/auth/sign-in`,
    { data: { email: process.env.TEST_USER_EMAIL, password: process.env.TEST_USER_PASSWORD } }
  )
  const body = await res.json()
  return body.token
}
