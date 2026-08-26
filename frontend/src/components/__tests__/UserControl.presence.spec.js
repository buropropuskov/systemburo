import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'
import { ONLINE_WINDOW_MINUTES } from '@/utils/presence'

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

const minutesAgo = (minutes) => new Date(NOW - minutes * 60_000).toISOString()

function seedUsers() {
  return [
    { id: 1, username: 'online_now', is_active: true, is_banned: false, last_seen: minutesAgo(1) },
    { id: 2, username: 'left_recently', is_active: true, is_banned: false, last_seen: minutesAgo(12) },
    { id: 3, username: 'never_seen', is_active: true, is_banned: false, last_seen: null },
    // Забаненный - со свежей активностью, но НЕ равной online_now: одинаковые метки
    // сделали бы проверку сортировки зависимой от стабильности sort, а не от ключа.
    { id: 4, username: 'banned_fresh', is_active: true, is_banned: true, last_seen: minutesAgo(2) },
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

// Ячейки колонки присутствия в порядке строк таблицы. У присутствующих вместо
// подписи стоит бейдж «Онлайн», поэтому текст ячейки читаем как есть.
const seenCells = w => w.findAll('[data-testid="users-row-seen"]')
const seenTexts = w => seenCells(w).map(c => c.text())
const onlineBadges = w => w.findAll('[data-testid="users-row-online-badge"]')
// Текст подсказки берём из aria-label якоря: пузырёк телепортируется в body и
// рендерится только на наведение.
const hintText = cell => cell.find('[data-testid="users-row-seen-hint"]').attributes('aria-label')
// Логин в списке показывается с собачкой (#1567) - в ожиданиях спеки её нет, срезаем.
const rowLogins = w => w.findAll('.user-login').map(el => el.text().replace(/^@/, ''))

describe('UserControl — колонка присутствия', () => {
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

  it('рисует статус и время последнего визита по каждой строке', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    // Порядок по умолчанию — по логину: banned_fresh, left_recently, never_seen, online_now.
    expect(rowLogins(wrapper)).toEqual(['banned_fresh', 'left_recently', 'never_seen', 'online_now'])
    // Присутствующий - бейджем, отсутствующие - давностью последнего визита.
    expect(seenTexts(wrapper)).toEqual(['2 мин.', '12 мин.', '-', 'Онлайн'])
    expect(onlineBadges(wrapper)).toHaveLength(1)
    expect(onlineBadges(wrapper)[0].classes()).toContain('badge--success')
  })

  it('забаненный со свежей активностью не получает бейдж', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    const bannedCell = seenCells(wrapper)[0]
    expect(bannedCell.find('[data-testid="users-row-online-badge"]').exists()).toBe(false)
    expect(bannedCell.text()).toBe('2 мин.')
    expect(hintText(bannedCell)).toContain('Был в сети')
  })

  it('подсказка ячейки даёт полное время, а не сокращение', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    const titles = seenCells(wrapper).map(c => hintText(c))
    expect(titles[3]).toContain('В сети. Последняя активность: 1 мин. назад')
    expect(titles[1]).toMatch(/Был в сети: 12 мин. назад \(\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}\)/)
    expect(titles[2]).toBe('Ни разу не заходил')

    // Нативного title на ячейке больше нет: он гас при каждом тике давности.
    expect(seenCells(wrapper)[0].attributes('title')).toBeUndefined()
  })

  it('счётчик в шапке считает присутствующих независимо от поиска', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    const counter = wrapper.find('[data-testid="users-online-count"]')
    expect(counter.text()).toBe('в сети: 1')

    wrapper.vm.userSearch = 'never'
    await flushPromises()
    expect(rowLogins(wrapper)).toEqual(['never_seen'])
    expect(counter.text()).toBe('в сети: 1')
  })

  it('режим «В сети» оставляет только присутствующих', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.onArchiveModeChange('online')
    await flushPromises()

    expect(rowLogins(wrapper)).toEqual(['online_now'])
    expect(wrapper.vm.showArchive).toBe(false)

    // Возврат к активным показывает всех снова, архив - только архивных.
    wrapper.vm.onArchiveModeChange('active')
    await flushPromises()
    expect(rowLogins(wrapper)).toHaveLength(4)
  })

  it('подпись футера отражает режим списка, а не «всего пользователей» под фильтром', async () => {
    wrapper = mountUserControl()
    await flushPromises()
    const footer = () => wrapper.find('.items-count').text()
    expect(footer()).toBe('Всего пользователей: 4')

    wrapper.vm.onArchiveModeChange('online')
    await flushPromises()
    expect(footer()).toBe('В сети: 1')

    wrapper.vm.onArchiveModeChange('archive')
    await flushPromises()
    expect(footer()).toBe('В архиве: 0')
  })

  it('сортировка по колонке ставит свежие визиты и «не заходил» в противоположные концы', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.sortBy('last_seen')
    await flushPromises()
    const firstPass = rowLogins(wrapper)
    expect(firstPass[0]).toBe('never_seen')
    expect(firstPass[firstPass.length - 1]).toBe('online_now')

    wrapper.vm.sortBy('last_seen')
    await flushPromises()
    const secondPass = rowLogins(wrapper)
    expect(secondPass[0]).toBe('online_now')
    expect(secondPass[secondPass.length - 1]).toBe('never_seen')
  })

  it('бейдж сменяется давностью по таймеру, без перезапроса списка', async () => {
    wrapper = mountUserControl()
    await flushPromises()
    expect(onlineBadges(wrapper)).toHaveLength(1)

    // Уводим время за окно онлайна, данные не меняем. Тик забирает Date.now(),
    // поэтому итоговое «сейчас» = NOW + 5 мин + 1 с тика.
    vi.setSystemTime(NOW + ONLINE_WINDOW_MINUTES * 60_000)
    vi.advanceTimersByTime(1000)
    await flushPromises()

    expect(onlineBadges(wrapper)).toHaveLength(0)
    expect(seenTexts(wrapper)[3]).toBe('6 мин. 1 сек.')
  })

  it('давность пересчитывается каждую секунду — иначе секунды врали бы', async () => {
    wrapper = mountUserControl()
    await flushPromises()
    // Забаненный: у него метка свежая, но бейджа нет, поэтому видна сама давность.
    expect(seenTexts(wrapper)[0]).toBe('2 мин.')

    vi.setSystemTime(NOW + 20_000)
    vi.advanceTimersByTime(1000)
    await flushPromises()
    expect(seenTexts(wrapper)[0]).toBe('2 мин. 21 сек.')
  })

  it('тихий опрос просит родителя перезагрузить список', async () => {
    wrapper = mountUserControl()
    await flushPromises()
    const before = wrapper.emitted('fetch-users')?.length || 0

    vi.advanceTimersByTime(60_000)
    await flushPromises()

    expect(wrapper.emitted('fetch-users').length).toBe(before + 1)
  })

  it('опрос молчит при открытой карточке пользователя', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.selectUser(wrapper.vm.sortedUsers[0])
    await flushPromises()
    const before = wrapper.emitted('fetch-users')?.length || 0

    vi.advanceTimersByTime(60_000)
    await flushPromises()

    expect(wrapper.emitted('fetch-users')?.length || 0).toBe(before)
  })

  it('опрос молчит в скрытой вкладке', async () => {
    wrapper = mountUserControl()
    await flushPromises()
    const before = wrapper.emitted('fetch-users')?.length || 0

    // jsdom держит hidden геттером на прототипе - подменяем own-свойством документа.
    let tabHidden = true
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => tabHidden })

    vi.advanceTimersByTime(60_000)
    await flushPromises()
    expect(wrapper.emitted('fetch-users')?.length || 0).toBe(before)

    // Возврат во вкладку возобновляет опрос.
    tabHidden = false
    vi.advanceTimersByTime(60_000)
    await flushPromises()
    expect(wrapper.emitted('fetch-users').length).toBe(before + 1)

    delete document.hidden
  })

  it('таймеры останавливаются при размонтировании', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.unmount()
    const after = wrapper.emitted('fetch-users')?.length || 0
    vi.advanceTimersByTime(180_000)
    expect(wrapper.emitted('fetch-users')?.length || 0).toBe(after)
    wrapper = null
  })
})
