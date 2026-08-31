import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const getNotificationPreferences = vi.fn()
const updateNotificationPreferences = vi.fn()
vi.mock('@/api/notificationPreferences', () => ({
  getNotificationPreferences: (...args) => getNotificationPreferences(...args),
  updateNotificationPreferences: (...args) => updateNotificationPreferences(...args),
}))

import NotificationSettingsView from '../NotificationSettingsView.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import { useDeletionsStore } from '@/stores/deletions'

function fixture() {
  return [
    {
      type_code: 'application_created', category: 'application',
      label: 'Заявка отправлена', description: 'Заявка принята и ждёт согласования.',
      mandatory: false, default_enabled: true, enabled: true,
    },
    {
      type_code: 'application_question', category: 'application',
      label: 'Новое обсуждение по заявке', description: 'По вашей заявке начали обсуждение.',
      mandatory: false, default_enabled: true, enabled: true,
    },
    {
      type_code: 'password_changed', category: 'security',
      label: 'Пароль изменён', description: 'Пароль вашей учётной записи изменён.',
      mandatory: true, default_enabled: true, enabled: true,
    },
    {
      type_code: 'user_banned', category: 'security',
      label: 'Учётная запись заблокирована', description: 'Вашу учётную запись заблокировали.',
      mandatory: true, default_enabled: true, enabled: true,
    },
  ]
}

async function mountView(data = fixture()) {
  getNotificationPreferences.mockResolvedValue(data)
  const wrapper = mount(NotificationSettingsView)
  await flushPromises()
  return wrapper
}

function toggleByTestId(wrapper, testId) {
  return wrapper.findAllComponents(ToggleSwitch).find((c) => c.attributes('data-testid') === testId)
}

async function emitToggle(wrapper, testId, value) {
  toggleByTestId(wrapper, testId).vm.$emit('update:modelValue', value)
  await wrapper.vm.$nextTick()
}

describe('NotificationSettingsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getNotificationPreferences.mockReset()
    updateNotificationPreferences.mockReset()
  })

  it('кнопка "Сохранить" неактивна без изменений', async () => {
    const wrapper = await mountView()
    const saveBtn = wrapper.get('[data-testid="notif-settings-save"]')
    expect(saveBtn.attributes('disabled')).toBeDefined()
  })

  it('обязательные типы показаны включёнными и задизейбленными - и категория, и тип', async () => {
    const wrapper = await mountView()
    expect(toggleByTestId(wrapper, 'item-toggle-password_changed').props('disabled')).toBe(true)
    expect(toggleByTestId(wrapper, 'item-toggle-password_changed').props('modelValue')).toBe(true)
    expect(toggleByTestId(wrapper, 'item-toggle-user_banned').props('disabled')).toBe(true)
    expect(toggleByTestId(wrapper, 'category-toggle-security').props('disabled')).toBe(true)
    expect(toggleByTestId(wrapper, 'category-toggle-security').props('modelValue')).toBe(true)
  })

  it('категория с выключенным тумблером выключает все необязательные типы внутри и не трогает обязательные', async () => {
    const wrapper = await mountView()

    await emitToggle(wrapper, 'category-toggle-application', false)

    expect(toggleByTestId(wrapper, 'item-toggle-application_created').props('modelValue')).toBe(false)
    expect(toggleByTestId(wrapper, 'item-toggle-application_question').props('modelValue')).toBe(false)
    // Категория security не задета - в ней нет необязательных типов.
    expect(toggleByTestId(wrapper, 'item-toggle-password_changed').props('modelValue')).toBe(true)
    expect(toggleByTestId(wrapper, 'item-toggle-user_banned').props('modelValue')).toBe(true)

    const saveBtn = wrapper.get('[data-testid="notif-settings-save"]')
    expect(saveBtn.attributes('disabled')).toBeUndefined()
  })

  it('в PUT уходят только изменённые типы', async () => {
    const wrapper = await mountView()
    updateNotificationPreferences.mockResolvedValue({})

    await emitToggle(wrapper, 'item-toggle-application_question', false)
    await wrapper.get('[data-testid="notif-settings-save"]').trigger('click')
    await flushPromises()

    expect(updateNotificationPreferences).toHaveBeenCalledTimes(1)
    expect(updateNotificationPreferences).toHaveBeenCalledWith([
      { type_code: 'application_question', enabled: false },
    ])
  })

  it('обязательный тип не попадает в тело запроса ни при каких действиях', async () => {
    const wrapper = await mountView()
    updateNotificationPreferences.mockResolvedValue({})

    // Прямой эмит в обход задизейбленного тумблера (defense-in-depth: диф на
    // save() обязан отфильтровать mandatory сам, а не полагаться только на UI).
    await emitToggle(wrapper, 'item-toggle-password_changed', false)
    await emitToggle(wrapper, 'category-toggle-security', false)
    // Настоящее изменение, чтобы кнопка "Сохранить" стала активна.
    await emitToggle(wrapper, 'item-toggle-application_created', false)

    // Обязательные тумблеры не сдвинулись даже прямым эмитом.
    expect(toggleByTestId(wrapper, 'item-toggle-password_changed').props('modelValue')).toBe(true)
    expect(toggleByTestId(wrapper, 'item-toggle-user_banned').props('modelValue')).toBe(true)

    await wrapper.get('[data-testid="notif-settings-save"]').trigger('click')
    await flushPromises()

    expect(updateNotificationPreferences).toHaveBeenCalledTimes(1)
    const [items] = updateNotificationPreferences.mock.calls[0]
    expect(items.some((i) => i.type_code === 'password_changed' || i.type_code === 'user_banned')).toBe(false)
    expect(items).toEqual([{ type_code: 'application_created', enabled: false }])
  })

  it('ошибка сохранения показывается через notify', async () => {
    const wrapper = await mountView()
    const store = useDeletionsStore()
    const notifySpy = vi.spyOn(store, 'notify')
    updateNotificationPreferences.mockRejectedValue(new Error('Сервер недоступен'))

    await emitToggle(wrapper, 'item-toggle-application_created', false)
    await wrapper.get('[data-testid="notif-settings-save"]').trigger('click')
    await flushPromises()

    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
  })

  it('успешное сохранение показывает уведомление через notify и снимает dirty', async () => {
    const wrapper = await mountView()
    const store = useDeletionsStore()
    const notifySpy = vi.spyOn(store, 'notify')
    updateNotificationPreferences.mockResolvedValue({})

    await emitToggle(wrapper, 'item-toggle-application_created', false)
    await wrapper.get('[data-testid="notif-settings-save"]').trigger('click')
    await flushPromises()

    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ bold: 'Настройки уведомлений' }))
    expect(wrapper.get('[data-testid="notif-settings-save"]').attributes('disabled')).toBeDefined()
  })

  it('разворачивает ответ, сгруппированный по категориям', async () => {
    const flat = fixture()
    const grouped = [
      { category: 'application', items: flat.filter((i) => i.category === 'application') },
      { category: 'security', items: flat.filter((i) => i.category === 'security') },
    ]
    const wrapper = await mountView(grouped)

    expect(wrapper.findAll('[data-testid^="item-toggle-"]')).toHaveLength(flat.length)
    expect(wrapper.text()).toContain('Заявка отправлена')
    expect(wrapper.text()).toContain('Учётная запись и безопасность')
  })
})
