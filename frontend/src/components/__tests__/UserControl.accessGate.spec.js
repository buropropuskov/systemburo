import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'
import { usePermissionsStore } from '@/stores/permissions'

vi.mock('@/api/settings', () => ({
  getPasswordPolicy: vi.fn().mockResolvedValue({
    min_length: 8, require_letter: true, require_digit: true,
  }),
}))
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}))
vi.mock('@/api/onboarding', () => ({ resetOnboardingForUser: vi.fn().mockResolvedValue({}) }))
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }))

const USERS = [{ id: 1, username: 'chop_kpp4', is_active: true }]

function mountUserControl() {
  return mount(UserControl, {
    props: { allUsers: USERS },
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
        $route: { path: '/admin/users', params: {} },
      },
    },
  })
}

// Карточка редактирования уезжает в body через Teleport, поэтому её кнопки
// ищутся по документу, а не по wrapper.
const accessButton = () => document.body.querySelector('[data-testid="user-access"]')
// Соседняя кнопка той же панели, у неё гейта нет. Стережёт от зелёного по
// ошибке: без неё «кнопки нет» читалось бы одинаково и когда её спрятало
// право, и когда карточка просто не открылась.
const cardOpened = () => [...document.body.querySelectorAll('.modal-header-actions button')]
  .some((b) => b.textContent.trim() === 'История')

async function openCard(wrapper) {
  await wrapper.findAll('.user-item')[0].trigger('click')
  await flushPromises()
}

// Окно прав доступа целиком стоит на permission.audit.manage (#1967): каталог
// ключей, эффективные права цели, роли и группы закрыты этим правом на бэкенде.
// Без права окно открывается пустым и с чередой отказов, поэтому вход в него
// показывается только тому, у кого право есть.
describe('UserControl — вход в окно прав доступа', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  afterEach(() => wrapper?.unmount())

  it('без permission.audit.manage кнопки нет, а сама карточка открыта', async () => {
    const permissions = usePermissionsStore()
    permissions.mode = 'normal'
    permissions.effective = { 'page.admin.users': { value: 'allow' } }

    wrapper = mountUserControl()
    await flushPromises()
    await openCard(wrapper)

    expect(cardOpened()).toBe(true)
    expect(accessButton()).toBeNull()
  })

  it('с permission.audit.manage кнопка появляется', async () => {
    const permissions = usePermissionsStore()
    permissions.mode = 'normal'
    permissions.effective = {
      'page.admin.users': { value: 'allow' },
      'permission.audit.manage': { value: 'allow' },
    }

    wrapper = mountUserControl()
    await flushPromises()
    await openCard(wrapper)

    expect(accessButton()).not.toBeNull()
    expect(accessButton().textContent.trim()).toBe('Права доступа')
  })

  it('администратору кнопка видна: бэкенд пускает его к правам без отдельного гранта', async () => {
    const permissions = usePermissionsStore()
    permissions.mode = 'admin'
    permissions.denied = new Set()

    wrapper = mountUserControl()
    await flushPromises()
    await openCard(wrapper)

    expect(accessButton()).not.toBeNull()
  })
})
