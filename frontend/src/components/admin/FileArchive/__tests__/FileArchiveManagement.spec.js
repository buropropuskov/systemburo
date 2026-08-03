import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const api = vi.hoisted(() => ({
  getArchiveSettings: vi.fn(),
  getArchiveStats: vi.fn(),
}))
vi.mock('@/api/fileArchive', () => api)

import FileArchiveManagement from '../FileArchiveManagement.vue'
import ArchiveStatusPanel from '../ArchiveStatusPanel.vue'
import ArchiveSettingsForm from '../ArchiveSettingsForm.vue'
import ArchiveDownloadPanel from '../ArchiveDownloadPanel.vue'
import ArchiveBackfillPanel from '../ArchiveBackfillPanel.vue'
import ArchiveFailuresList from '../ArchiveFailuresList.vue'
import RefreshButton from '@/components/RefreshButton.vue'
import { useDeletionsStore } from '@/stores/deletions'

const emptyStats = {
  used_bytes: 0,
  free_bytes: 0,
  file_count: 0,
  periods: [],
  statuses: {},
  disk: {
    total_bytes: 0, free_bytes: 0, archive_bytes: 0, uploads_bytes: 0,
    database_bytes: 0, logs_bytes: 0, other_bytes: 0, partitions: [],
  },
  generated_at: '2026-07-31T00:00:00Z',
}

function mountCmp() {
  return mount(FileArchiveManagement, {
    global: {
      stubs: {
        RefreshButton: true,
        ArchiveSettingsForm: true,
        // Скачивание/бэкфилл/ошибки (срез C4) - тяжёлые панели со своим API и
        // жизненным циклом, у каждой отдельный spec-файл; здесь проверяется
        // только каркас вкладок, поэтому глушим их до заглушек.
        ArchiveDownloadPanel: true,
        ArchiveBackfillPanel: true,
        ArchiveFailuresList: true,
      },
    },
  })
}

describe('FileArchiveManagement', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    api.getArchiveStats.mockResolvedValue(emptyStats)
    const del = useDeletionsStore()
    vi.spyOn(del, 'notify').mockImplementation(() => {})
  })

  it('загружает настройки при монтировании и показывает статус «Активен»', async () => {
    api.getArchiveSettings.mockResolvedValue({ enabled: true, dir_template: '{год}' })
    const w = mountCmp()
    await flushPromises()

    expect(api.getArchiveSettings).toHaveBeenCalled()
    expect(w.text()).toContain('Активен')
  })

  it('показывает статус «Неактивен» для выключенного рубильника', async () => {
    api.getArchiveSettings.mockResolvedValue({ enabled: false })
    const w = mountCmp()
    await flushPromises()

    expect(w.text()).toContain('Неактивен')
  })

  // jsdom не даёт живого layout-пересчёта на отсоединённом от document дереве (mount()
  // без attachTo), поэтому DOMWrapper.isVisible() (getComputedStyle) там ненадёжен -
  // проверяем непосредственно инлайновый display, который v-show выставляет сам.
  function isShown(wrapper) {
    return wrapper.element.style.display !== 'none'
  }

  it('переключает вкладки Обзор/Настройки/Ошибки по клику', async () => {
    api.getArchiveSettings.mockResolvedValue({ enabled: false })
    const w = mountCmp()
    await flushPromises()

    const tabs = w.findAll('.file-archive__tab')
    expect(tabs).toHaveLength(3)
    expect(w.findComponent(ArchiveStatusPanel).exists()).toBe(true)
    expect(w.findComponent(ArchiveSettingsForm).exists()).toBe(true)
    // «Обзор» с C4 несёт ещё скачивание ZIP и бэкфилл - тоже смонтированы сразу.
    expect(w.findComponent(ArchiveDownloadPanel).exists()).toBe(true)
    expect(w.findComponent(ArchiveBackfillPanel).exists()).toBe(true)
    // Все три секции смонтированы одновременно (v-show, не v-if) - несохранённые
    // правки в ArchiveSettingsForm не теряются при переключении на «Обзор»/«Ошибки».
    // Порядок в DOM фиксирован: 0-Обзор, 1-Настройки, 2-Ошибки.
    const panels = w.findAll('.file-archive__panel')
    expect(panels).toHaveLength(3)
    expect(isShown(panels[0])).toBe(true)
    expect(isShown(panels[1])).toBe(false)

    // «Настройки» с C3 - реальный ArchiveSettingsForm (заглушен), а не текст-плейсхолдер.
    await tabs[1].trigger('click')
    expect(isShown(panels[1])).toBe(true)
    expect(w.findComponent(ArchiveSettingsForm).exists()).toBe(true)

    // «Ошибки» с C4 - реальный ArchiveFailuresList (заглушен), а не текст-плейсхолдер.
    await tabs[2].trigger('click')
    expect(isShown(panels[2])).toBe(true)
    expect(w.findComponent(ArchiveFailuresList).exists()).toBe(true)
  })

  it('при ошибке загрузки показывает сообщение и уведомляет через notify', async () => {
    api.getArchiveSettings.mockRejectedValue(new Error('Сервер недоступен'))
    const w = mountCmp()
    await flushPromises()

    expect(w.find('.file-archive__error').text()).toBe('Сервер недоступен')
    expect(useDeletionsStore().notify).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'error' }),
    )
  })

  it('обновляет настройки по клику RefreshButton (@refresh)', async () => {
    api.getArchiveSettings.mockResolvedValue({ enabled: false })
    const w = mountCmp()
    await flushPromises()
    expect(api.getArchiveSettings).toHaveBeenCalledTimes(1)

    await w.findComponent(RefreshButton).vm.$emit('refresh')
    await flushPromises()
    expect(api.getArchiveSettings).toHaveBeenCalledTimes(2)
  })

  it('RefreshButton на вкладке «Обзор» обновляет и настройки, и сводку архива', async () => {
    api.getArchiveSettings.mockResolvedValue({ enabled: false })
    const w = mountCmp()
    await flushPromises()
    expect(api.getArchiveStats).toHaveBeenCalledTimes(1)

    await w.findComponent(RefreshButton).vm.$emit('refresh')
    await flushPromises()
    expect(api.getArchiveSettings).toHaveBeenCalledTimes(2)
    expect(api.getArchiveStats).toHaveBeenCalledTimes(2)
  })

  it('RefreshButton на другой вкладке не дёргает сводку архива (панель размонтирована)', async () => {
    api.getArchiveSettings.mockResolvedValue({ enabled: false })
    const w = mountCmp()
    await flushPromises()
    await w.findAll('.file-archive__tab')[1].trigger('click')
    expect(api.getArchiveStats).toHaveBeenCalledTimes(1)

    await w.findComponent(RefreshButton).vm.$emit('refresh')
    await flushPromises()
    expect(api.getArchiveStats).toHaveBeenCalledTimes(1)
  })
})
