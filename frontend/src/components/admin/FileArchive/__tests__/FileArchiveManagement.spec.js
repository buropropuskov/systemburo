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
import ArchiveSettingsView from '../ArchiveSettingsView.vue'
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
        ArchiveSettingsView: true,
        // Скачивание/бэкфилл/ошибки (срез C4) - тяжёлые панели со своим API и
        // жизненным циклом, у каждой отдельный spec-файл; здесь проверяется
        // только каркас вкладок, поэтому глушим их до заглушек.
        ArchiveDownloadPanel: true,
        ArchiveBackfillPanel: true,
        // Заглушка списка ошибок умеет refresh: каркас зовёт его у активной
        // вкладки, и голая заглушка без метода уронила бы обработчик.
        ArchiveFailuresList: { template: '<div />', methods: { refresh() {} } },
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

  it('оставляет две вкладки и держит наблюдение на «Обзоре»', async () => {
    api.getArchiveSettings.mockResolvedValue({ enabled: false })
    const w = mountCmp()
    await flushPromises()

    // Вкладки «Настройки» больше нет: раскладку и пороги задаёт команда на сервере,
    // а не форма в вебе (#1615). Показ действующих значений переехал на «Обзор».
    const tabs = w.findAll('.file-archive__tab')
    expect(tabs).toHaveLength(2)
    expect(tabs.map(t => t.text())).toEqual(['Обзор', 'Лента'])

    expect(w.findComponent(ArchiveStatusPanel).exists()).toBe(true)
    expect(w.findComponent(ArchiveDownloadPanel).exists()).toBe(true)
    expect(w.findComponent(ArchiveBackfillPanel).exists()).toBe(true)
    expect(w.findComponent(ArchiveSettingsView).exists()).toBe(true)

    const panels = w.findAll('.file-archive__panel')
    expect(panels).toHaveLength(2)
    expect(isShown(panels[0])).toBe(true)
    expect(isShown(panels[1])).toBe(false)

    await tabs[1].trigger('click')
    expect(isShown(panels[1])).toBe(true)
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

  it('обновление работает на видимой вкладке, а не на скрытой', async () => {
    // Обе секции смонтированы (v-show), но обновляется только та, что на экране:
    // иначе кнопка обновления на «Ошибках» перезапрашивала бы ещё и сводку места.
    api.getArchiveSettings.mockResolvedValue({ enabled: false })
    const w = mountCmp()
    await flushPromises()
    expect(api.getArchiveStats).toHaveBeenCalledTimes(1)

    await w.findAll('.file-archive__tab')[1].trigger('click')
    await w.findComponent(RefreshButton).vm.$emit('refresh')
    await flushPromises()
    expect(api.getArchiveStats).toHaveBeenCalledTimes(1)

    await w.findAll('.file-archive__tab')[0].trigger('click')
    await w.findComponent(RefreshButton).vm.$emit('refresh')
    await flushPromises()
    expect(api.getArchiveStats).toHaveBeenCalledTimes(2)
  })
})
