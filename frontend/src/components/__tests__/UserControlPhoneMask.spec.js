import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'

// Телефон в карточке пользователя: маска не должна выбрасывать курсор в конец
// при правке середины и залипать на стирании разделителя.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}))
vi.mock('@/api/settings', () => ({
  getPasswordPolicy: vi.fn().mockResolvedValue({ min_length: 8, require_letter: true, require_digit: true }),
}))
vi.mock('@/api/onboarding', () => ({ resetOnboardingForUser: vi.fn().mockResolvedValue({}) }))
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }))

beforeEach(() => {
  setActivePinia(createPinia())
  localStorage.clear()
})

/** Эмулирует ввод в поле телефона: браузер уже изменил value, дальше идёт обработчик. */
function fireInput(vm, value, caret, inputType, modelKey = 'newUser') {
  const input = document.createElement('input')
  input.type = 'tel'
  input.value = value
  input.setSelectionRange(caret, caret)
  vm.onPhoneInput({ target: input, inputType }, modelKey)
  return input
}

describe('UserControl - маска телефона', () => {
  const mountControl = () => mount(UserControl, { props: { allUsers: [] } })

  it('маскирует ввод по мере набора', () => {
    const w = mountControl()
    const input = fireInput(w.vm, '916', 3, 'insertText')

    expect(input.value).toBe('+7 (916)')
    expect(w.vm.newUser.phone).toBe('+7 (916)')
  })

  it('держит каретку у той же цифры при правке середины', () => {
    const w = mountControl()
    w.vm.newUser.phone = '+7 (916) 123 45-67'
    // Пользователь заменил "1" в "123" на "9": каретка стоит сразу за новой цифрой
    const input = fireInput(w.vm, '+7 (916) 923 45-67', 10, 'insertText')

    expect(input.value).toBe('+7 (916) 923 45-67')
    expect(input.selectionStart).toBe(10)
  })

  it('маскирует телефон и в карточке выбранного пользователя', () => {
    const w = mountControl()
    w.vm.selectedUser = { id: 1, username: 'ivanov', phone: '' }
    const input = fireInput(w.vm, '8916', 4, 'insertText', 'selectedUser')

    expect(input.value).toBe('+7 (916)')
    expect(w.vm.selectedUser.phone).toBe('+7 (916)')
  })

  it('одно нажатие Backspace стирает цифру, даже если под кареткой был разделитель', () => {
    const w = mountControl()
    w.vm.newUser.phone = '+7 (916) 123 45-67'
    // Браузер съел ")", набор цифр не изменился
    const input = fireInput(w.vm, '+7 (916 123 45-67', 7, 'deleteContentBackward')

    expect(input.value).toBe('+7 (911) 234 56-7')
    expect(w.vm.newUser.phone).toBe('+7 (911) 234 56-7')
  })
})
