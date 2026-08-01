import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const api = vi.hoisted(() => ({
  getArchiveSettings: vi.fn(),
  updateArchiveSettings: vi.fn(),
  getArchiveTokens: vi.fn(),
  previewArchivePath: vi.fn(),
}))
vi.mock('@/api/fileArchive', () => api)

import ArchiveSettingsForm from '../ArchiveSettingsForm.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import ConfirmationModal from '@/components/ConfirmationModal.vue'
import { useDeletionsStore } from '@/stores/deletions'

const SETTINGS = {
  enabled: true,
  dir_template: '{год}/{организация}',
  file_template: '{тип}',
  quota_bytes: 0,
  min_free_bytes: 2147483648,
  warn_percent: 80,
  recheck_days: 30,
  freeze_after_days: 30,
  zip_max_bytes: 2147483648,
}
const TOKENS = [
  { key: 'год', label: 'Год', group: 'Дата', example: '2026', dir_allowed: true, file_allowed: true },
  { key: 'организация', label: 'Организация', group: 'Организация', example: 'Мегобари', dir_allowed: true, file_allowed: true },
  { key: 'тип', label: 'Тип вложения', group: 'Вложение', example: 'Заявка', dir_allowed: false, file_allowed: true },
]
const PREVIEW = {
  levels: ['2026', 'Мегобари'],
  file_name: 'Заявка.xlsx',
  rel_path: '2026/Мегобари/Заявка.xlsx',
  synthetic: true,
  application_number: '',
  dir_problems: [],
  file_problems: [],
}

function mountForm() {
  setActivePinia(createPinia())
  vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {})
  return mount(ArchiveSettingsForm, { global: { stubs: { Teleport: true } } })
}

describe('ArchiveSettingsForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.getArchiveSettings.mockResolvedValue(SETTINGS)
    api.getArchiveTokens.mockResolvedValue(TOKENS)
    api.previewArchivePath.mockResolvedValue(PREVIEW)
    api.updateArchiveSettings.mockResolvedValue(SETTINGS)
  })

  it('загружает настройки и токены при монтировании; без правок кнопка "Сохранить" выключена', async () => {
    const w = mountForm()
    await flushPromises()

    expect(api.getArchiveSettings).toHaveBeenCalled()
    expect(api.getArchiveTokens).toHaveBeenCalled()
    expect(w.find('[data-testid="asf-save"]').attributes('disabled')).toBeDefined()
  })

  it('правка шаблона папки включает "Сохранить" и обновляет полный путь превью', async () => {
    const w = mountForm()
    await flushPromises()

    const dirInput = w.findAll('[data-testid="tpf-input"]')[0]
    await dirInput.setValue('{год}/новый')
    expect(w.find('[data-testid="asf-save"]').attributes('disabled')).toBeUndefined()
    expect(w.find('[data-testid="asf-full-preview"]').text()).toContain('2026/Мегобари/Заявка.xlsx')
  })

  it('один debounce-запрос превью на правку обоих шаблонов подряд, не по одному на поле', async () => {
    vi.useFakeTimers()
    try {
      const w = mountForm()
      await flushPromises()
      const callsAfterMount = api.previewArchivePath.mock.calls.length

      const [dirInput, fileInput] = w.findAll('[data-testid="tpf-input"]')
      await dirInput.setValue('{год}/A')
      await fileInput.setValue('{тип}-B')
      expect(api.previewArchivePath.mock.calls.length).toBe(callsAfterMount) // до паузы бэк не дёргаем

      await vi.advanceTimersByTimeAsync(400)
      expect(api.previewArchivePath.mock.calls.length).toBe(callsAfterMount + 1)
    } finally {
      vi.useRealTimers()
    }
  })

  it('выключение архива требует подтверждения перед сохранением', async () => {
    const w = mountForm()
    await flushPromises()

    await w.findComponent(ToggleSwitch).vm.$emit('update:modelValue', false)
    await w.find('[data-testid="asf-save"]').trigger('click')

    expect(api.updateArchiveSettings).not.toHaveBeenCalled()
    expect(w.findComponent(ConfirmationModal).props('show')).toBe(true)

    await w.findComponent(ConfirmationModal).vm.$emit('confirm')
    await flushPromises()
    expect(api.updateArchiveSettings).toHaveBeenCalledWith({ enabled: false })
  })

  it('смена шаблона папки требует подтверждения перед сохранением', async () => {
    const w = mountForm()
    await flushPromises()

    const dirInput = w.findAll('[data-testid="tpf-input"]')[0]
    await dirInput.setValue('{организация}/{год}')
    await w.find('[data-testid="asf-save"]').trigger('click')

    expect(api.updateArchiveSettings).not.toHaveBeenCalled()
    expect(w.findComponent(ConfirmationModal).props('show')).toBe(true)
  })

  it('правка числового порога сохраняется без подтверждения', async () => {
    const w = mountForm()
    await flushPromises()

    const warnInput = w.find('[data-testid="asf-warn-percent"]')
    await warnInput.setValue(90)
    await w.find('[data-testid="asf-save"]').trigger('click')
    await flushPromises()

    expect(w.findComponent(ConfirmationModal).props('show')).toBe(false)
    expect(api.updateArchiveSettings).toHaveBeenCalledWith({ warnPercent: 90 })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('подпись под рубильником следует за его состоянием', async () => {
    // Статический текст «пока выключено» под включённым тумблером утверждает
    // обратное тому, что делает система: администратор решает, что бланки не
    // пишутся, хотя они пишутся.
    api.getArchiveSettings.mockResolvedValue({ ...SETTINGS, enabled: true })
    const w = mount(ArchiveSettingsForm)
    await flushPromises()

    expect(w.text()).toContain('Включено')
    expect(w.text()).not.toContain('Пока выключено')

    await w.findComponent(ToggleSwitch).vm.$emit('update:modelValue', false)
    await w.vm.$nextTick()
    expect(w.text()).toContain('Пока выключено')
  })
})
