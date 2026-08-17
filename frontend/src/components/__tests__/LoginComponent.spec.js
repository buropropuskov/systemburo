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
    expect(SFC).not.toMatch(/@\/assets\/(?!icons\/)[^'")]+\.(png|jpe?g|webp)/)
  })

  // Плоская заливка читалась пустым экраном: глубину даёт разница планов, а не
  // их наличие. Замок держит все три - убрав средний, легко не заметить потерю.
  it('держит три плана пейзажа с разной прозрачностью', () => {
    const wrapper = mountLogin()
    const scene = wrapper.find('.login-scene')
    expect(scene.exists()).toBe(true)

    const depth = {}
    scene.element.querySelectorAll('[fill]').forEach((el) => {
      const plane = el.getAttribute('fill').match(/ls(Far|Mid|Near)/)
      if (plane) depth[plane[1]] = el.getAttribute('opacity')
    })
    ;['Far', 'Mid', 'Near'].forEach((plane) => {
      expect(depth[plane], `план ${plane} пропал или потерял opacity`).toBeTruthy()
    })
    // Дальний светлее ближнего - это и есть воздушная перспектива.
    expect(Number(depth.Far)).toBeLessThan(Number(depth.Mid))
    expect(Number(depth.Mid)).toBeLessThan(Number(depth.Near))
    wrapper.unmount()
  })

  // Кроны деревьев на холмах владелец прочёл теми же кругами, что просил убрать:
  // «вот ты вернул круги, но не убрал круги, которые добавил на волны». Планы
  // пейзажа рисуются одними кривыми, круглого на них нет.
  it('держит планы пейзажа кривыми, без круглых крон', () => {
    const wrapper = mountLogin()
    const scene = wrapper.find('.login-scene')
    expect(scene.exists()).toBe(true)

    scene.element.querySelectorAll('g[fill]').forEach((group) => {
      const plane = group.getAttribute('fill').match(/ls(Mid|Near)/)
      if (!plane) return
      const round = [...group.querySelectorAll('ellipse, circle')]
      expect(round.length, `на плане ${plane[1]} снова круглые кроны`).toBe(0)
      const tags = [...group.children].map((el) => el.tagName.toLowerCase())
      expect(tags, `план ${plane[1]} рисуется не только кривыми`).toEqual(['path'])
    })
    wrapper.unmount()
  })

  it('оживляет фон линиями поверх волн', () => {
    const wrapper = mountLogin()
    expect(wrapper.findAll('.login-lines__group').length).toBeGreaterThanOrEqual(3)
    wrapper.unmount()
  })

  // Круги плавали на этом экране с самого начала - их убрали заодно с
  // добавленными поверх, хотя просили снять только добавленные. Замок держит
  // исходные семь поимённо: переписывая фон, их легко потерять снова, и ни один
  // тест выше этого не заметит.
  it('держит семь исходных плавающих кругов', () => {
    const wrapper = mountLogin()
    const shapes = wrapper.findAll('.floating-shape')
    expect(shapes.length, 'исходные круги пропали с экрана входа').toBe(7)
    for (let n = 1; n <= 7; n += 1) {
      expect(
        wrapper.find(`.shape-${n}`).exists(),
        `круг shape-${n} пропал - у каждого свой размер и место`,
      ).toBe(true)
      expect(SFC, `правило .shape-${n} пропало из стилей`).toMatch(
        new RegExp(`\\.shape-${n} \\{[^}]*width:`),
      )
    }
    wrapper.unmount()
  })

  // Покачивание на top/margin дало бы пересчёт раскладки вместо композита.
  it('анимирует фон только transform и opacity', () => {
    const frames = SFC.match(/@keyframes (line-sway|float) \{[\s\S]*?\n {4}\}/g)
    expect(frames, 'кадры анимаций фона не найдены').not.toBeNull()
    expect(frames.length).toBe(2)
    frames.forEach((frame) => {
      const props = [...frame.matchAll(/^\s{12}([a-z-]+):/gm)].map((m) => m[1])
      expect(props.length).toBeGreaterThan(0)
      props.forEach((prop) => expect(['transform', 'opacity']).toContain(prop))
    })
  })

  // Сетка, пейзаж и линии - три наложенных слоя. При равных z-index
  // порядок держался бы только очерёдностью в шаблоне: перестановка блоков
  // роняла бы пейзаж под сетку молча, без единой ошибки.
  it('держит слои фона явной лестницей z-index', () => {
    const layers = ['.login-pattern', '.login-scene', '.login-lines', '.login-background']
    const depths = layers.map((sel) => {
      const rule = SFC.match(new RegExp(`\\${sel} \\{[^}]+\\}`))
      expect(rule, `слой ${sel} пропал из стилей`).not.toBeNull()
      const z = rule[0].match(/z-index: (\d+);/)
      expect(z, `${sel} остался без z-index`).not.toBeNull()
      return Number(z[1])
    })
    depths.forEach((z, i) => {
      if (i > 0) expect(z, `${layers[i]} не выше ${layers[i - 1]}`).toBeGreaterThan(depths[i - 1])
    })
  })

  it('снимает движение при prefers-reduced-motion', () => {
    const block = SFC.match(/@media \(prefers-reduced-motion: reduce\) \{[\s\S]*?\n {4}\}\n/)
    expect(block, 'блок prefers-reduced-motion пропал').not.toBeNull()
    ;['.login-lines__group', '.floating-shape']
      .forEach((sel) => expect(block[0]).toContain(sel))
  })
})
