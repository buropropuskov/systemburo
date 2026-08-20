import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { mount } from '@vue/test-utils'
import PasswordRecoveryModal from '@/components/PasswordRecoveryModal.vue'

vi.mock('@/stores/contacts', () => ({
  useContactsStore: () => ({
    email: 'buro@example.ru',
    phone: '+7 (910) 000 00-00',
    fetch: vi.fn(),
  }),
}))

const writeText = vi.fn().mockResolvedValue(undefined)

function mountModal() {
  return mount(PasswordRecoveryModal, { props: { show: true }, attachTo: document.body })
}

beforeEach(() => {
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
  writeText.mockClear()
})

afterEach(() => {
  document.documentElement.removeAttribute('data-theme')
  document.body.innerHTML = ''
})

describe('PasswordRecoveryModal — светлый остров', () => {
  it('окно остаётся светлым при тёмной теме системы', () => {
    document.documentElement.setAttribute('data-theme', 'dark')
    mountModal()

    expect(document.querySelector('.base-modal-overlay').getAttribute('data-theme')).toBe('light')
  })

  it('радиус приходит пропом, а не оформлением из родителя', () => {
    // Оверрайды через :deep до телепортированного контента не достают (#1076),
    // поэтому единственное живое место радиуса - инлайн-переменная BaseModal.
    mountModal()

    expect(document.querySelector('.base-modal').style.getPropertyValue('--base-modal-radius')).toBe('45px')
  })

  it('окно не полагается на мёртвые оверрайды секций BaseModal', () => {
    const source = readFileSync(
      path.join(import.meta.dirname, '..', 'PasswordRecoveryModal.vue'),
      'utf8'
    )

    expect(source).not.toMatch(/:deep\(\s*\.base-modal/)
  })
})

describe('PasswordRecoveryModal — уведомление о копировании', () => {
  it('пилюля появляется по клику и живёт светлым островом', async () => {
    document.documentElement.setAttribute('data-theme', 'dark')
    mountModal()

    await document.querySelector('[data-testid="recovery-copy-email"]').click()
    await new Promise((resolve) => setTimeout(resolve, 0))

    const pill = document.querySelector('[data-testid="recovery-notification"]')
    expect(writeText).toHaveBeenCalledWith('buro@example.ru')
    expect(pill.textContent.trim()).toBe('E-mail скопирован')
    expect(pill.getAttribute('data-theme')).toBe('light')
  })

  it('пилюля встаёт над окном, а не на фолбэчной высоте', async () => {
    mountModal()
    const modal = document.querySelector('.base-modal')
    // jsdom не считает раскладку, поэтому кромку окна задаём сами.
    modal.getBoundingClientRect = () => ({ top: 300, bottom: 600, left: 0, right: 440, width: 440, height: 300 })

    await document.querySelector('[data-testid="recovery-copy-email"]').click()
    await new Promise((resolve) => setTimeout(resolve, 0))

    // 768 (jsdom innerHeight) - 300 (верх окна) + 14 (просвет)
    expect(document.querySelector('[data-testid="recovery-notification"]').style.bottom).toBe('482px')
  })

  it('телефон копируется своим текстом', async () => {
    mountModal()

    await document.querySelector('[data-testid="recovery-copy-phone"]').click()
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(writeText).toHaveBeenCalledWith('+7 (910) 000 00-00')
    expect(
      document.querySelector('[data-testid="recovery-notification"]').textContent.trim()
    ).toBe('Номер телефона скопирован')
  })
})
