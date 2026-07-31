import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'
import { resetUserLockout } from '@/api/users'
import { useDeletionsStore } from '@/stores/deletions'

vi.mock('@/api/settings', () => ({
  getPasswordPolicy: vi.fn().mockResolvedValue({ min_length: 8, require_letter: true, require_digit: true }),
}))
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}))
vi.mock('@/api/onboarding', () => ({ resetOnboardingForUser: vi.fn().mockResolvedValue({}) }))
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

const NOW = new Date('2026-07-30T12:00:00Z').getTime()
const inMinutes = (m) => new Date(NOW + m * 60_000).toISOString()

function seedUsers() {
  return [
    { id: 1, username: 'locked_user', is_active: true, locked_until: inMinutes(15), lockout_level: 3 },
    { id: 2, username: 'free_user', is_active: true, locked_until: null, lockout_level: 0 },
    // Срок вышел: строка обязана выглядеть как у незаблокированного.
    { id: 3, username: 'expired_user', is_active: true, locked_until: inMinutes(-5), lockout_level: 2 },
  ]
}

function mountUserControl(allUsers = seedUsers()) {
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

const rows = w => w.findAll('.user-item')
const lockoutBadges = w => w.findAll('[data-testid="users-row-lockout"]')
// Карточка редактирования уезжает в body через Teleport, поэтому её кнопки
// ищутся не по wrapper, а по документу.
const resetButton = () => document.body.querySelector('[data-testid="user-reset-lockout"]')

describe('UserControl — блокировка входа и её снятие', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.useFakeTimers()
    vi.setSystemTime(NOW)
  })
  afterEach(() => {
    wrapper?.unmount()
    vi.useRealTimers()
  })

  it('отметку в списке несёт только тот, у кого срок ещё не вышел', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    const badges = lockoutBadges(wrapper)
    expect(badges).toHaveLength(1)
    expect(badges[0].text()).toContain('вход заблокирован')
    // Первая строка - locked_user (сортировка по логину не меняет состав отметок).
    const owner = rows(wrapper).find(r => r.find('[data-testid="users-row-lockout"]').exists())
    expect(owner.text()).toContain('locked_user')
  })

  it('кнопка снятия есть в карточке заблокированного и отсутствует у остальных', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.selectUser({ id: 2, username: 'free_user', is_active: true, locked_until: null })
    await flushPromises()
    expect(resetButton()).toBeNull()

    wrapper.vm.selectUser({ id: 1, username: 'locked_user', is_active: true, locked_until: inMinutes(15) })
    await flushPromises()
    expect(resetButton()).not.toBeNull()
  })

  it('клик снимает блокировку, сообщает об этом и гасит отметку без перезапроса списка', async () => {
    resetUserLockout.mockResolvedValue({ reset: true })
    wrapper = mountUserControl()
    await flushPromises()
    const notify = vi.spyOn(useDeletionsStore(), 'notify')

    wrapper.vm.selectUser({ id: 1, username: 'locked_user', is_active: true, locked_until: inMinutes(15) })
    await flushPromises()
    resetButton().click()
    await flushPromises()

    expect(resetUserLockout).toHaveBeenCalledWith('locked_user')
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'locked_user' }))
    expect(resetButton()).toBeNull()

    // Строка списка обновляется точечным событием: рефетч при открытой карточке
    // пере-резолвил бы selectedUser и затёр незасохранённый ввод формы.
    expect(wrapper.emitted('fetch-users') || []).toHaveLength(1) // только маунт
    expect(wrapper.emitted('user-updated')).toEqual([
      [{ username: 'locked_user', locked_until: null, lockout_level: 0 }],
    ])
  })

  it('отказ бэка показывается ошибкой, а отметка остаётся', async () => {
    resetUserLockout.mockRejectedValue(new Error('Недостаточно прав'))
    wrapper = mountUserControl()
    await flushPromises()
    const notify = vi.spyOn(useDeletionsStore(), 'notify')

    wrapper.vm.selectUser({ id: 1, username: 'locked_user', is_active: true, locked_until: inMinutes(15) })
    await flushPromises()
    resetButton().click()
    await flushPromises()

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error', bold: 'Недостаточно прав' }))
    expect(resetButton()).not.toBeNull()
    expect(wrapper.emitted('user-updated')).toBeUndefined()
  })

  it('отметка гаснет сама, когда срок истёк, без перезагрузки страницы', async () => {
    wrapper = mountUserControl([
      { id: 1, username: 'locked_user', is_active: true, locked_until: inMinutes(1), lockout_level: 1 },
    ])
    await flushPromises()
    expect(lockoutBadges(wrapper)).toHaveLength(1)

    // Тик присутствия (раз в секунду) двигает точку отсчёта - на нём и держится отметка.
    vi.setSystemTime(NOW + 61_000)
    vi.advanceTimersByTime(1000)
    await flushPromises()

    expect(lockoutBadges(wrapper)).toHaveLength(0)
  })
})
