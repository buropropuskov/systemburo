import fs from 'node:fs'
import path from 'node:path'

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ setTokens: vi.fn() }) }))
vi.mock('@/stores/contacts', () => ({ useContactsStore: () => ({ fetch: vi.fn(), email: '', phone: '' }) }))

import LoginComponent from '@/components/LoginComponent.vue'
import { apiRequest } from '@/api/client'

// Response-подобная заглушка: handleSubmit читает ok/status/headers.get/json.
function resp(status, headers = {}, body = {}) {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: '',
    headers: { get: (k) => (Object.prototype.hasOwnProperty.call(headers, k) ? headers[k] : null) },
    json: async () => body,
    text: async () => JSON.stringify(body),
  }
}

function mountLogin() {
  return mount(LoginComponent, {
    global: {
      stubs: { PasswordRecoveryModal: true },
      mocks: { $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } },
    },
  })
}

async function submit(wrapper) {
  wrapper.vm.formData.username = 'ivanov'
  wrapper.vm.formData.password = 'secret'
  const p = wrapper.vm.handleSubmit()
  // Проходим внутренний 100мс delay handleSubmit + showErrorWithDelay + микротаски.
  await vi.advanceTimersByTimeAsync(300)
  await p
  await nextTick()
}

describe('LoginComponent — 401 счётчик попыток', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('показывает остаток попыток из заголовка X-Auth-Attempts-Remaining', async () => {
    apiRequest.mockResolvedValue(resp(401, { 'X-Auth-Attempts-Remaining': '7' }))
    const wrapper = mountLogin()
    await submit(wrapper)

    expect(wrapper.vm.errors.general).toBe('Неверный логин или пароль. Осталось попыток: 7')
    wrapper.unmount()
  })

  it('без заголовка счётчика показывает общий текст', async () => {
    apiRequest.mockResolvedValue(resp(401, {}))
    const wrapper = mountLogin()
    await submit(wrapper)

    expect(wrapper.vm.errors.general).toBe('Неверный логин или пароль')
    wrapper.unmount()
  })
})

describe('LoginComponent — 429 таймер', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('на 429 запускает обратный отсчёт из Retry-After и блокирует кнопку', async () => {
    apiRequest.mockResolvedValue(resp(429, { 'Retry-After': '120' }))
    const wrapper = mountLogin()
    await submit(wrapper)

    expect(wrapper.vm.cooldownSeconds).toBe(120)
    expect(wrapper.vm.isCoolingDown).toBe(true)
    expect(wrapper.vm.displayError).toBe('Слишком много попыток. Повторите через 2:00')

    const btn = wrapper.find('[data-testid="login-button-submit"]')
    expect(btn.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  // Верхняя ступень лестницы - час (#1600). "60:00" читалось бы как минуты,
  // поэтому час выводится отдельным разрядом.
  it('часовой кулдаун показывается часами, а не 60 минутами', async () => {
    apiRequest.mockResolvedValue(resp(429, { 'Retry-After': '3600' }))
    const wrapper = mountLogin()
    await submit(wrapper)

    expect(wrapper.vm.displayError).toBe('Слишком много попыток. Повторите через 1:00:00')
    wrapper.unmount()
  })

  it('таймер убывает и по нулю разблокирует', async () => {
    apiRequest.mockResolvedValue(resp(429, { 'Retry-After': '2' }))
    const wrapper = mountLogin()
    await submit(wrapper)

    expect(wrapper.vm.cooldownSeconds).toBe(2)
    await vi.advanceTimersByTimeAsync(2000)
    expect(wrapper.vm.cooldownSeconds).toBe(0)
    expect(wrapper.vm.isCoolingDown).toBe(false)
    wrapper.unmount()
  })

  it('без Retry-After использует fallback 60с', async () => {
    apiRequest.mockResolvedValue(resp(429, {}))
    const wrapper = mountLogin()
    await submit(wrapper)

    expect(wrapper.vm.cooldownSeconds).toBe(60)
    wrapper.unmount()
  })

  it('на кнопке входа только таймер MM:SS, без слова «Подождите»', async () => {
    apiRequest.mockResolvedValue(resp(429, { 'Retry-After': '120' }))
    const wrapper = mountLogin()
    await submit(wrapper)

    expect(wrapper.vm.getButtonText).toBe('2:00')
    const btn = wrapper.find('[data-testid="login-button-submit"]')
    expect(btn.text()).toBe('2:00')
    expect(btn.text()).not.toContain('Подождите')
    wrapper.unmount()
  })

  it('кнопка получает класс cooling (серый вид) во время таймера', async () => {
    apiRequest.mockResolvedValue(resp(429, { 'Retry-After': '30' }))
    const wrapper = mountLogin()
    await submit(wrapper)

    const btn = wrapper.find('[data-testid="login-button-submit"]')
    expect(btn.classes()).toContain('cooling')
    expect(btn.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('персистит кулдаун в localStorage и восстанавливает после перезагрузки (F5)', async () => {
    apiRequest.mockResolvedValue(resp(429, { 'Retry-After': '120' }))
    const w1 = mountLogin()
    await submit(w1)
    expect(localStorage.getItem('loginCooldownUntil')).toBeTruthy()
    // unmount эмулирует уход со страницы - персист НЕ должен стереться.
    w1.unmount()
    expect(localStorage.getItem('loginCooldownUntil')).toBeTruthy()

    // Новый монтаж (эмуляция F5): блокировка восстановлена, кнопка заблокирована.
    const w2 = mountLogin()
    await nextTick()
    expect(w2.vm.isCoolingDown).toBe(true)
    expect(w2.vm.cooldownSeconds).toBeGreaterThan(0)
    expect(w2.find('[data-testid="login-button-submit"]').attributes('disabled')).toBeDefined()
    w2.unmount()
  })

  it('истёкший кулдаун в localStorage не блокирует новый вход', async () => {
    // Просроченная метка (в прошлом) должна игнорироваться и очищаться.
    localStorage.setItem('loginCooldownUntil', String(Date.now() - 5000))
    const wrapper = mountLogin()
    await nextTick()

    expect(wrapper.vm.isCoolingDown).toBe(false)
    expect(localStorage.getItem('loginCooldownUntil')).toBeNull()
    wrapper.unmount()
  })
})

// Сбой на стороне сервера (недоступная база, исчерпанный пул) приходит на вход 500,
// а не 401: дело не в пароле. Форма разбирает такой ответ сама - у /login есть
// исключение из редиректа на страницу ошибки в client.js.
describe('LoginComponent — 500 сбой на стороне сервера', () => {
  const SERVER_MESSAGE = 'Вход временно недоступен из-за ошибки на сервере. Повторите попытку позже.'

  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('показывает текст из конверта ошибки, а не [object Object]', async () => {
    apiRequest.mockResolvedValue(resp(500, {}, { success: false, error: SERVER_MESSAGE }))
    const wrapper = mountLogin()
    await submit(wrapper)

    expect(wrapper.vm.errors.general).toBe(SERVER_MESSAGE)
    expect(wrapper.find('[data-testid="login-error-message"]').text()).toBe(SERVER_MESSAGE)
    wrapper.unmount()
  })

  it('не запускает таймер блокировки - повторить можно сразу', async () => {
    apiRequest.mockResolvedValue(resp(500, {}, { success: false, error: SERVER_MESSAGE }))
    const wrapper = mountLogin()
    await submit(wrapper)

    expect(wrapper.vm.isCoolingDown).toBe(false)
    expect(localStorage.getItem('loginCooldownUntil')).toBeNull()
    expect(wrapper.find('[data-testid="login-button-submit"]').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
})

// Фон входа рисуется градиентами: снимок неизвестного происхождения оттуда убран,
// и вернуться картинкой он не должен. jsdom стили не считает, поэтому читаем SFC.
describe('LoginComponent: фон экрана', () => {
  const SFC = fs.readFileSync(
    path.join(import.meta.dirname, '..', 'LoginComponent.vue'),
    'utf8',
  )

  it('рисует фон градиентами, а не файлом', () => {
    const layer = SFC.match(/\.login-pattern \{[^}]+\}/)
    expect(layer, 'слой .login-pattern пропал из стилей').not.toBeNull()
    expect(layer[0]).toMatch(/gradient\(/)
    expect(layer[0]).not.toMatch(/url\(/)
  })

  it('не ссылается на растровые ассеты вне каталога иконок', () => {
    expect(SFC).not.toMatch(/@\/assets\/[^/'")]+\.(png|jpe?g|webp)/)
  })
})
