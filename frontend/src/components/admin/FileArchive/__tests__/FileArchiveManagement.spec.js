import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const api = vi.hoisted(() => ({
  getArchiveSettings: vi.fn(),
}))
vi.mock('@/api/fileArchive', () => api)

import FileArchiveManagement from '../FileArchiveManagement.vue'
import RefreshButton from '@/components/RefreshButton.vue'
import { useDeletionsStore } from '@/stores/deletions'

function mountCmp() {
  return mount(FileArchiveManagement, {
    global: { stubs: { RefreshButton: true } },
  })
}

describe('FileArchiveManagement', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
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

  it('переключает вкладки Обзор/Настройки/Ошибки по клику', async () => {
    api.getArchiveSettings.mockResolvedValue({ enabled: false })
    const w = mountCmp()
    await flushPromises()

    const tabs = w.findAll('.file-archive__tab')
    expect(tabs).toHaveLength(3)
    expect(w.find('.file-archive__panel').text()).toContain('Обзор')

    await tabs[1].trigger('click')
    expect(w.find('.file-archive__panel').text()).toContain('Настройки')

    await tabs[2].trigger('click')
    expect(w.find('.file-archive__panel').text()).toContain('Ошибки')
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
})
