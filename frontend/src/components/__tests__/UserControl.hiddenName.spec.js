import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'
import { apiRequest } from '@/api/client'
import { useUiStore } from '@/stores/ui'

// ФИО работника, не давшего согласия на обработку персональных данных, сервер не
// присылает и помечает признаком pd_hidden (#1567). Скрыты и ФИО, и контакты.
// Карточка редактирования
// обязана отличать это от незаполненного поля: иначе правка должности или почты
// уходит на сервер с пустым ФИО и стирает настоящее.

vi.mock('@/api/settings', () => ({
  getPasswordPolicy: vi.fn().mockResolvedValue({ min_length: 8, require_letter: true, require_digit: true }),
}))
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}))
vi.mock('@/api/onboarding', () => ({ resetOnboardingForUser: vi.fn().mockResolvedValue({}) }))
const revokeUserConsent = vi.fn()
vi.mock('@/api/pdConsent', () => ({ revokeUserConsent: (...a) => revokeUserConsent(...a) }))
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }))
vi.mock('@/api/users', () => ({
  bulkArchiveUsers: vi.fn(),
  bulkRestoreUsers: vi.fn(),
  bulkUpdateUsersType: vi.fn(),
  bulkAssignUsersOrganization: vi.fn(),
  bulkAssignUsersCompany: vi.fn(),
  bulkBanUsers: vi.fn(),
  bulkUnbanUsers: vi.fn(),
  resetUserLockout: vi.fn(),
}))

const hiddenUser = {
  id: 1,
  username: 'silent_user',
  is_active: true,
  last_name: null,
  first_name: null,
  middle_name: null,
  pd_hidden: true,
  position: 'Инженер',
  email: 'e@example.com',
}

const openUser = {
  id: 2,
  username: 'open_user',
  is_active: true,
  last_name: 'Иванов',
  first_name: 'Иван',
  middle_name: 'Иванович',
  pd_hidden: false,
  position: 'Инженер',
}

function mountUserControl(allUsers) {
  return mount(UserControl, {
    props: { allUsers },
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
        $route: { path: '/admin/users', params: {} },
      },
    },
  })
}

/** Тело последнего PUT на /users/:username/info. */
function lastInfoPayload() {
  const call = [...apiRequest.mock.calls].reverse()
    .find(([url, opts]) => /\/users\/.+\/info$/.test(url) && opts?.method === 'PUT')
  return call ? JSON.parse(call[1].body) : null
}

describe('UserControl — скрытое до согласия ФИО', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiRequest.mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) })
  })
  afterEach(() => wrapper?.unmount())

  it('в строке списка вместо прочерка показывает логин', async () => {
    wrapper = mountUserControl([hiddenUser])
    await flushPromises()

    expect(wrapper.text()).toContain('silent_user')
    expect(wrapper.find('.name-col').text()).not.toBe('-')
  })

  it('правка соседнего поля не отправляет пустое ФИО', async () => {
    wrapper = mountUserControl([hiddenUser])
    await flushPromises()

    await wrapper.vm.updateUserInfo({ ...wrapper.vm.filteredUsers[0], position: 'Главный инженер' })

    const payload = lastInfoPayload()
    expect(payload).not.toBeNull()
    expect(payload.position).toBe('Главный инженер')
    expect('last_name' in payload).toBe(false)
    expect('first_name' in payload).toBe(false)
    expect('middle_name' in payload).toBe(false)
    // Почта и телефон - такие же персональные данные: их тоже нельзя затирать.
    expect('email' in payload).toBe(false)
    expect('phone' in payload).toBe(false)
  })

  it('у обычного работника ФИО по-прежнему отправляется', async () => {
    wrapper = mountUserControl([openUser])
    await flushPromises()

    await wrapper.vm.updateUserInfo({ ...wrapper.vm.filteredUsers[0], position: 'Главный инженер' })

    const payload = lastInfoPayload()
    expect(payload.last_name).toBe('Иванов')
    expect(payload.first_name).toBe('Иван')
    expect('email' in payload).toBe(true)
  })
})

describe('UserControl — согласие на обработку данных в карточке', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiRequest.mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) })
  })
  afterEach(() => wrapper?.unmount())

  it('в колонке ФИО у скрытого пишет «без согласия», а не повторяет логин', async () => {
    wrapper = mountUserControl([{ ...hiddenUser, consent_required: true, consent_granted: false }])
    await flushPromises()

    const marker = wrapper.find('[data-testid="users-row-no-consent"]')
    expect(marker.exists()).toBe(true)
    expect(marker.text()).toBe('без согласия')
    // Логин уже стоит в соседней колонке - дублировать его незачем.
    expect(wrapper.find('.user-row .name-col').text()).not.toContain('silent_user')
  })

  it('пока согласие не запрашивают, метки нет: его нет вообще ни у кого', async () => {
    wrapper = mountUserControl([{ ...openUser, pd_hidden: false }])
    await flushPromises()

    expect(wrapper.find('[data-testid="users-row-no-consent"]').exists()).toBe(false)
    expect(wrapper.find('.user-row .name-col').text()).toContain('Иванов')
  })

  it('подтвердившего меткой не помечает', async () => {
    wrapper = mountUserControl([{ ...openUser, consent_required: true, consent_granted: true }])
    await flushPromises()

    expect(wrapper.find('[data-testid="users-row-no-consent"]').exists()).toBe(false)
  })

  it('в карточке показывает дату согласия', async () => {
    wrapper = mountUserControl([openUser])
    await flushPromises()

    expect(wrapper.vm.consentStateLabel({ consent_granted: true, consent_at: '2026-07-12T08:30:00Z' }))
      .toBe('Дано 12.07.2026')
    expect(wrapper.vm.consentStateLabel({ consent_granted: false, consent_required: true }))
      .toBe('Не дано')
    // Пока запрос выключен, «не дано» само по себе ничего не означает.
    expect(wrapper.vm.consentStateLabel({ consent_granted: false, consent_required: false }))
      .toContain('не запрашивается')
  })
})

describe('UserControl — поиск по логину с собачкой', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiRequest.mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) })
  })
  afterEach(() => wrapper?.unmount())

  it('находит человека и когда собачку скопировали в запрос', async () => {
    wrapper = mountUserControl([openUser, { ...hiddenUser, username: 'other_user' }])
    await flushPromises()

    wrapper.vm.userSearch = '@open_user'
    await flushPromises()

    expect(wrapper.vm.filteredUsers.map(u => u.username)).toEqual(['open_user'])
  })
})

describe('UserControl — отзыв согласия администратором', () => {
  let wrapper
  const granted = { ...openUser, consent_granted: true, consent_at: '2026-07-12T08:30:00Z', consent_required: true }

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiRequest.mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) })
    revokeUserConsent.mockResolvedValue({})
  })
  afterEach(() => wrapper?.unmount())

  it('спрашивает подтверждение и отзывает', async () => {
    wrapper = mountUserControl([granted])
    await flushPromises()
    const confirmSpy = vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true)
    wrapper.vm.selectedUser = { ...granted }

    await wrapper.vm.revokeConsent(granted)
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ confirmText: 'Отозвать' }))
    expect(revokeUserConsent).toHaveBeenCalledWith('open_user')
    // Карточка обязана сразу показать новое состояние, не дожидаясь перезагрузки списка.
    expect(wrapper.vm.selectedUser.consent_granted).toBe(false)
    expect(wrapper.emitted('fetch-users')).toBeTruthy()
  })

  it('отказ в подтверждении ничего не отзывает', async () => {
    wrapper = mountUserControl([granted])
    await flushPromises()
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(false)

    await wrapper.vm.revokeConsent(granted)

    expect(revokeUserConsent).not.toHaveBeenCalled()
  })

  it('кнопка отзыва есть только у того, кто согласие давал', async () => {
    wrapper = mountUserControl([granted])
    await flushPromises()

    // Карточку открывает selectUser, а сама она уезжает в body через Teleport.
    wrapper.vm.selectUser({ ...granted })
    await flushPromises()
    expect(document.body.querySelector('[data-testid="user-consent-revoke"]')).not.toBeNull()

    wrapper.vm.selectUser({ ...granted, consent_granted: false })
    await flushPromises()
    expect(document.body.querySelector('[data-testid="user-consent-revoke"]')).toBeNull()
  })
})
