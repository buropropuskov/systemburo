// Сквозной сценарий «Регистрация + авторизация» в сервисе Dockee.
// Основано на маршрутах /auth/sign-up и /auth/sign-in из dokkee-backend и
// компонентах страниц регистрации/входа из dokkee-frontend.
//
// Стек: Playwright Test (@playwright/test).

import { test, expect } from '@playwright/test'

const BASE = process.env.DOKKEE_URL || 'http://localhost:8080'
const API  = process.env.API_URL    || 'http://localhost:8000'

test.describe('Аутентификация', () => {
  test('регистрация нового пользователя и вход в систему', async ({ page, request }) => {
    const ts = Date.now()
    const user = {
      username:    `e2e_${ts}`,
      password:    'StrongPass123!',
      first_name:  'Тест',
      last_name:   'Пользователь',
      middle_name: '',
      email:       `e2e_${ts}@example.com`,
      phone:       `+7999${String(ts).slice(-7)}`,
    }

    // Регистрация через API (готовим пользователя для входа через UI)
    const signUp = await request.post(`${API}/auth/sign-up`, { data: user })
    expect(signUp.status()).toBe(200)
    const { id } = await signUp.json()
    expect(typeof id).toBe('number')

    // Вход через UI
    await page.goto(`${BASE}/login`)
    //возможны изменения// — селекторы зависят от вёрстки LoginPage.vue,
    // уточнить после реализации формы входа во фронтенде.
    await page.getByLabel('Email').fill(user.email)
    await page.getByLabel('Пароль').fill(user.password)
    await page.getByRole('button', { name: /войти/i }).click()

    // После входа пользователь попадает в сервис
    await expect(page).toHaveURL(/\/service/)
    await expect(page.getByText(user.first_name)).toBeVisible()
  })

  test('вход с неверным паролем показывает сообщение об ошибке', async ({ page }) => {
    await page.goto(`${BASE}/login`)
    await page.getByLabel('Email').fill('nobody@example.com')
    await page.getByLabel('Пароль').fill('wrong')
    await page.getByRole('button', { name: /войти/i }).click()

    await expect(page.getByText(/неверн/i)).toBeVisible()
  })
})
