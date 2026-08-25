import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ChangePasswordModal from '@/components/ChangePasswordModal.vue'

vi.mock('@/api/settings', () => ({
  getPasswordPolicy: vi.fn().mockResolvedValue({
    min_length: 8,
    require_letter: true,
    require_uppercase: false,
    require_lowercase: false,
    require_digit: true,
    require_special: false,
  }),
}))

const changeOwnPassword = vi.fn()
vi.mock('@/api/users', () => ({
  changeOwnPassword: (...args) => changeOwnPassword(...args),
}))

const notify = vi.fn()
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}))

const clearTokens = vi.fn()
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ clearTokens }),
}))

const push = vi.fn()

// Стаб объявляет ВСЕ пробрасываемые пропсы: обязательный режим (#1911) выражается
// именно ими (closable/closeOnOverlay), и не объявленные тут они утекли бы в attrs,
// где проверить их нечем.
const BaseModalStub = {
  template: '<div><slot /><slot name="actions" /></div>',
  props: ['show', 'title', 'width', 'contentTestid', 'closable', 'closeOnOverlay', 'zIndex'],
}

function mountModal(props = {}) {
  return mount(ChangePasswordModal, {
    props: { show: false, ...props },
    global: {
      mocks: {
        $router: { push, currentRoute: { value: { path: '/personal-cabinet' } } },
      },
      stubs: {
        // BaseModal телепортирует в body - в jsdom проще подменить оболочкой,
        // содержимое слотов при этом остаётся в дереве компонента.
        BaseModal: BaseModalStub,
      },
    },
  })
}

/** Открывает модалку и заполняет форму. */
async function fillForm(wrapper, { current, next, repeat }) {
  await wrapper.setProps({ show: true })
  await flushPromises()
  wrapper.vm.currentPassword = current
  wrapper.vm.newPassword = next
  wrapper.vm.repeatPassword = repeat ?? next
  await flushPromises()
}

describe('ChangePasswordModal', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('не даёт отправить, пока новый пароль не проходит политику', async () => {
    const wrapper = mountModal()
    await fillForm(wrapper, { current: 'oldpass12345', next: 'short1' })

    expect(wrapper.vm.canSubmit).toBe(false)
    const submit = wrapper.find('[data-testid="cp-submit"]')
    expect(submit.attributes('disabled')).toBeDefined()
  })

  it('не даёт отправить, пока повтор не совпал', async () => {
    const wrapper = mountModal()
    await fillForm(wrapper, { current: 'oldpass12345', next: 'newpass12345', repeat: 'newpass1234' })

    expect(wrapper.vm.canSubmit).toBe(false)
  })

  it('требует текущий пароль', async () => {
    const wrapper = mountModal()
    await fillForm(wrapper, { current: '', next: 'newpass12345' })

    expect(wrapper.vm.canSubmit).toBe(false)
  })

  it('после успешной смены чистит сессию и уводит на вход', async () => {
    changeOwnPassword.mockResolvedValue({ ok: true })
    const wrapper = mountModal()
    await fillForm(wrapper, { current: 'oldpass12345', next: 'newpass12345' })

    expect(wrapper.vm.canSubmit).toBe(true)
    await wrapper.vm.submit()
    await flushPromises()

    expect(changeOwnPassword).toHaveBeenCalledWith('oldpass12345', 'newpass12345')
    expect(clearTokens).toHaveBeenCalled()
    expect(push).toHaveBeenCalledWith('/')
  })

  it('показывает текст ошибки сервера и оставляет сессию живой', async () => {
    changeOwnPassword.mockResolvedValue({
      ok: false,
      json: vi.fn().mockResolvedValue({ message: 'Текущий пароль указан неверно' }),
    })
    const wrapper = mountModal()
    await fillForm(wrapper, { current: 'wrongpass123', next: 'newpass12345' })

    await wrapper.vm.submit()
    await flushPromises()

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      bold: 'Текущий пароль указан неверно',
      type: 'error',
    }))
    expect(clearTokens).not.toHaveBeenCalled()
    expect(push).not.toHaveBeenCalled()
  })

  it('чеклист требований считается по политике с сервера', async () => {
    const wrapper = mountModal()
    await fillForm(wrapper, { current: 'oldpass12345', next: 'abcdefgh' })

    const rules = wrapper.vm.rules
    expect(rules.find((r) => r.key === 'min_length').ok).toBe(true)
    expect(rules.find((r) => r.key === 'digit').ok).toBe(false)
    expect(rules.some((r) => r.key === 'special')).toBe(false)
  })
})

// Обязательная смена пароля (#1911): пока флаг поднят, сервер отвечает отказом на
// всё, кроме самой смены. Закрываемое окно оставило бы человека перед пустым
// экраном, поэтому закрыть его нечем - но выход обязан остаться.
describe('ChangePasswordModal в обязательном режиме', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('окно не закрывается ни крестиком, ни по затемнению, ни Escape', async () => {
    const wrapper = mountModal({ show: true, mandatory: true })
    await flushPromises()

    const modal = wrapper.findComponent(BaseModalStub)
    // closable=false снимает и крестик, и обработчик Escape внутри BaseModal.
    expect(modal.props('closable')).toBe(false)
    expect(modal.props('closeOnOverlay')).toBe(false)
  })

  it('вместо отмены показывает выход и объясняет причину', async () => {
    const wrapper = mountModal({ show: true, mandatory: true })
    await flushPromises()

    expect(wrapper.text()).not.toContain('Отмена')
    expect(wrapper.find('[data-testid="cp-reason"]').exists()).toBe(true)

    await wrapper.find('[data-testid="cp-logout"]').trigger('click')
    expect(wrapper.emitted('logout')).toHaveLength(1)
  })

  it('в обычном режиме окно закрываемо и выхода в нём нет', async () => {
    const wrapper = mountModal({ show: true })
    await flushPromises()

    const modal = wrapper.findComponent(BaseModalStub)
    expect(modal.props('closable')).toBe(true)
    expect(modal.props('closeOnOverlay')).toBe(true)
    expect(wrapper.find('[data-testid="cp-logout"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="cp-reason"]').exists()).toBe(false)
  })
})
