import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import AdminSettings from '@/views/AdminSettings.vue'

const updateSetting = vi.fn().mockResolvedValue({})
vi.mock('@/api/settings', () => ({
  getSettings: vi.fn().mockResolvedValue([
    { key: 'password.min_length', value: '8' },
    { key: 'password.rotation_enabled', value: 'false' },
    { key: 'password.rotation_days', value: '90' },
    { key: 'password.rotation_notify_days_before', value: '7' },
    { key: 'password.force_change_on_next_login', value: 'true' },
  ]),
  updateSetting: (...args) => updateSetting(...args),
}))

const apiRequest = vi.fn()
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}))

const notify = vi.fn()
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}))

/** Ответ ручки состояния плановой смены. */
function statusResponse(overrides = {}) {
  return {
    ok: true,
    json: vi.fn().mockResolvedValue({
      data: {
        mail_configured: false,
        enabled: false,
        rotation_days: 90,
        eligible: 12,
        without_email: 3,
        expired: 5,
        expiring_soon: 2,
        next_run_at: '2026-08-12T04:00:00Z',
        ...overrides,
      },
    }),
  }
}

async function mountSettings(statusOverrides = {}) {
  apiRequest.mockImplementation((path) => {
    if (path === '/settings/password-rotation/status') return Promise.resolve(statusResponse(statusOverrides))
    return Promise.resolve({ ok: true, json: vi.fn().mockResolvedValue({}) })
  })
  const wrapper = mount(AdminSettings, {
    global: { stubs: { teleport: true, WorkScheduleTab: true, BaseDropdown: true } },
  })
  wrapper.vm.activeSection = 'security'
  await flushPromises()
  return wrapper
}

describe('AdminSettings: плановая смена паролей', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('без настроенной почты не даёт включить смену и объясняет причину', async () => {
    const wrapper = await mountSettings({ mail_configured: false })

    wrapper.vm.toggleRotation()
    await flushPromises()

    expect(wrapper.vm.settings.password_rotation_enabled).toBe(false)
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      bold: 'Сначала настройте почту',
      type: 'error',
    }))
  })

  it('с настроенной почтой переключатель работает', async () => {
    const wrapper = await mountSettings({ mail_configured: true })

    wrapper.vm.toggleRotation()
    await flushPromises()

    expect(wrapper.vm.settings.password_rotation_enabled).toBe(true)
    expect(notify).not.toHaveBeenCalled()
  })

  it('показывает, скольких работников затронет ближайший прогон', async () => {
    const wrapper = await mountSettings({ mail_configured: true })
    const block = wrapper.find('[data-testid="rotation-status"]')

    expect(block.text()).toContain('5')
    expect(block.text()).toContain('12')
    expect(block.text()).toContain('3')
    expect(block.text()).toContain('адреса проставляет бюро')
  })

  it('без почты вместо чисел показывает предупреждение', async () => {
    const wrapper = await mountSettings({ mail_configured: false })
    const block = wrapper.find('[data-testid="rotation-status"]')

    expect(block.text()).toContain('Почта не настроена')
    expect(block.text()).not.toContain('Ближайшая проверка')
  })

  it('сохранение шлёт все четыре ключа ротации', async () => {
    const wrapper = await mountSettings({ mail_configured: true })
    wrapper.vm.settings.password_rotation_enabled = true
    wrapper.vm.settings.password_rotation_days = 60

    await wrapper.vm.saveSecuritySettings()
    await flushPromises()

    const keys = updateSetting.mock.calls.map(([key]) => key)
    expect(keys).toContain('password.rotation_enabled')
    expect(keys).toContain('password.rotation_days')
    expect(keys).toContain('password.rotation_notify_days_before')
    expect(keys).toContain('password.force_change_on_next_login')
    expect(updateSetting).toHaveBeenCalledWith('password.rotation_days', '60')
  })

  it('переключатель обязательной смены не трогается при выключенной ротации', async () => {
    const wrapper = await mountSettings({ mail_configured: true })
    wrapper.vm.settings.password_rotation_enabled = false
    const before = wrapper.vm.settings.password_force_change_on_next_login

    wrapper.vm.toggleForceChange()
    await flushPromises()

    expect(wrapper.vm.settings.password_force_change_on_next_login).toBe(before)
  })

  it('дата ближайшего прогона показывается человеческим форматом', async () => {
    const wrapper = await mountSettings({ mail_configured: true })
    expect(wrapper.vm.nextRotationRunText).toMatch(/\d{2}\.\d{2}\.\d{4}/)
  })
})

describe('AdminSettings: ручной прогон смены паролей', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('кнопка прогона видна только при настроенной почте', async () => {
    const withMail = await mountSettings({ mail_configured: true })
    expect(withMail.find('[data-testid="rotation-run-button"]').exists()).toBe(true)

    const withoutMail = await mountSettings({ mail_configured: false })
    expect(withoutMail.find('[data-testid="rotation-run-button"]').exists()).toBe(false)
  })

  it('первый клик открывает подтверждение, а не запускает смену', async () => {
    const wrapper = await mountSettings({ mail_configured: true })

    await wrapper.find('[data-testid="rotation-run-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.vm.confirmRotation).toBe(true)
    // Ни одного запроса на запуск: действие обрывает сессии всей организации и
    // не должно срабатывать с одного клика.
    const runCalls = apiRequest.mock.calls.filter(([path]) => path === '/settings/password-rotation/run')
    expect(runCalls).toHaveLength(0)
  })

  it('подтверждение запускает прогон и сообщает об очереди писем', async () => {
    const wrapper = await mountSettings({ mail_configured: true })
    apiRequest.mockImplementation((path) => {
      if (path === '/settings/password-rotation/status') return Promise.resolve(statusResponse({ mail_configured: true }))
      if (path === '/settings/password-rotation/run') return Promise.resolve({ ok: true, json: vi.fn().mockResolvedValue({}) })
      return Promise.resolve({ ok: true, json: vi.fn().mockResolvedValue({}) })
    })

    await wrapper.vm.runRotationNow()
    await flushPromises()

    const runCalls = apiRequest.mock.calls.filter(([path]) => path === '/settings/password-rotation/run')
    expect(runCalls).toHaveLength(1)
    expect(runCalls[0][1]).toMatchObject({ method: 'POST' })
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'Смена паролей запущена' }))
    expect(wrapper.vm.confirmRotation).toBe(false)
  })

  it('отказ сервера показывается текстом, а не молча', async () => {
    const wrapper = await mountSettings({ mail_configured: true })
    apiRequest.mockImplementation((path) => {
      if (path === '/settings/password-rotation/status') return Promise.resolve(statusResponse({ mail_configured: true }))
      if (path === '/settings/password-rotation/run') {
        return Promise.resolve({
          ok: false,
          json: vi.fn().mockResolvedValue({ message: 'Смена паролей уже выполняется' }),
        })
      }
      return Promise.resolve({ ok: true, json: vi.fn().mockResolvedValue({}) })
    })

    await wrapper.vm.runRotationNow()
    await flushPromises()

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      bold: 'Смена паролей уже выполняется',
      type: 'error',
    }))
  })
})
