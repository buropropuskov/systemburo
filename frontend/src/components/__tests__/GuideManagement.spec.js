import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const api = vi.hoisted(() => ({
  listAllGuideSections: vi.fn(),
  updateGuideSection: vi.fn(),
  uploadGuideFile: vi.fn(),
  deleteGuideFile: vi.fn(),
  downloadGuideFile: vi.fn(),
}))
vi.mock('@/api/guide', () => api)

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: vi.fn(() => () => {}),
  confirmIfAnyDirty: vi.fn().mockResolvedValue(true),
}))

import GuideManagement from '../GuideManagement.vue'
import { confirmIfAnyDirty } from '@/utils/dirtyTracker'
import { useDeletionsStore } from '@/stores/deletions'
import { useUiStore } from '@/stores/ui'

function sections() {
  return [
    {
      role: 'user',
      title: 'Пользователь',
      lead: 'Лид пользователя',
      items: ['Пункт 1', 'Пункт 2'],
      file: {
        name: 'guide.pdf',
        ext: '.pdf',
        mime_type: 'application/pdf',
        size: 1048576,
        updated_at: '2026-06-18T10:00:00Z',
        download_url: '/api/guide/sections/user/download',
      },
    },
    { role: 'guard', title: 'Охранник', lead: 'Лид охранника', items: ['Охрана'], file: null },
    { role: 'admin', title: 'Администратор', lead: 'Лид админа', items: ['Админ'], file: null },
  ]
}

function mountCmp() {
  return mount(GuideManagement, {
    global: { stubs: { RefreshButton: true, LoaderSpinner: true, FileTypeIcon: true } },
    attachTo: document.body,
  })
}

function setFiles(input, files) {
  Object.defineProperty(input.element, 'files', { value: files, configurable: true })
}

describe('GuideManagement', () => {
  let del
  let ui
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    api.listAllGuideSections.mockResolvedValue(sections())
    del = useDeletionsStore()
    vi.spyOn(del, 'notify').mockImplementation(() => {})
    ui = useUiStore()
    vi.spyOn(ui, 'confirm').mockResolvedValue(true)
  })

  it('загружает разделы, рисует список и футер «Всего: 3»', async () => {
    const w = mountCmp()
    await flushPromises()
    expect(api.listAllGuideSections).toHaveBeenCalled()
    expect(w.findAll('[data-testid="guide-row"]')).toHaveLength(3)
    expect(w.find('.table-footer').text()).toContain('Всего: 3')
  })

  it('по клику на раздел заполняет редактор lead и items', async () => {
    const w = mountCmp()
    await flushPromises()
    await w.findAll('[data-testid="guide-row"]')[0].trigger('click')
    expect(w.find('[data-testid="guide-lead"]').element.value).toBe('Лид пользователя')
    expect(w.findAll('[data-testid="guide-item-input"]')).toHaveLength(2)
  })

  it('размер файла раздела считает общий formatBytes, а не своя копия', async () => {
    const w = mountCmp()
    await flushPromises()
    await w.findAll('[data-testid="guide-row"]')[0].trigger('click')
    // 1048576 байт: общий формат даёт «1.0 МБ», снятая локальная копия давала «1,0 МБ»
    expect(w.find('.file-card__meta').text()).toContain('1.0 МБ')
  })

  it('кнопка «Сохранить» неактивна без изменений', async () => {
    const w = mountCmp()
    await flushPromises()
    await w.findAll('[data-testid="guide-row"]')[0].trigger('click')
    expect(w.find('[data-testid="guide-save"]').attributes('disabled')).toBeDefined()
  })

  it('добавляет и удаляет пункт', async () => {
    const w = mountCmp()
    await flushPromises()
    await w.findAll('[data-testid="guide-row"]')[0].trigger('click')
    await w.find('[data-testid="guide-add-item"]').trigger('click')
    expect(w.findAll('[data-testid="guide-item-input"]')).toHaveLength(3)
    await w.findAll('[data-testid="guide-item-remove"]')[0].trigger('click')
    expect(w.findAll('[data-testid="guide-item-input"]')).toHaveLength(2)
  })

  it('сохраняет раздел: PUT с обрезанными непустыми items + уведомление', async () => {
    api.updateGuideSection.mockResolvedValue({
      role: 'admin', title: 'Администратор', lead: 'Новый лид', items: ['Админ'], file: null,
    })
    const w = mountCmp()
    await flushPromises()
    await w.findAll('[data-testid="guide-row"]')[2].trigger('click')
    await w.find('[data-testid="guide-lead"]').setValue('Новый лид')
    // пустой пункт должен отфильтроваться на сохранении
    await w.find('[data-testid="guide-add-item"]').trigger('click')
    const save = w.find('[data-testid="guide-save"]')
    expect(save.attributes('disabled')).toBeUndefined()
    await save.trigger('click')
    await flushPromises()
    expect(api.updateGuideSection).toHaveBeenCalledWith('admin', { lead: 'Новый лид', items: ['Админ'] })
    expect(del.notify).toHaveBeenCalled()
  })

  it('отклоняет не-PDF и не вызывает upload', async () => {
    const w = mountCmp()
    await flushPromises()
    await w.findAll('[data-testid="guide-row"]')[1].trigger('click')
    const input = w.find('[data-testid="guide-file-input"]')
    setFiles(input, [new File(['hi'], 'a.txt', { type: 'text/plain' })])
    await input.trigger('change')
    await flushPromises()
    expect(api.uploadGuideFile).not.toHaveBeenCalled()
    expect(del.notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
  })

  it('загружает PDF: вызывает uploadGuideFile с ролью и файлом', async () => {
    api.uploadGuideFile.mockResolvedValue({
      role: 'guard', title: 'Охранник', lead: 'Лид охранника', items: ['Охрана'],
      file: { name: 'g.pdf', ext: '.pdf', size: 100, updated_at: '2026-06-18T10:00:00Z', download_url: '/api/guide/sections/guard/download' },
    })
    const w = mountCmp()
    await flushPromises()
    await w.findAll('[data-testid="guide-row"]')[1].trigger('click')
    const input = w.find('[data-testid="guide-file-input"]')
    const pdf = new File(['%PDF-1.4'], 'g.pdf', { type: 'application/pdf' })
    setFiles(input, [pdf])
    await input.trigger('change')
    await flushPromises()
    expect(api.uploadGuideFile).toHaveBeenCalledWith('guard', pdf)
  })

  it('удаляет файл после подтверждения', async () => {
    api.deleteGuideFile.mockResolvedValue({
      role: 'user', title: 'Пользователь', lead: 'Лид пользователя', items: ['Пункт 1', 'Пункт 2'], file: null,
    })
    const w = mountCmp()
    await flushPromises()
    await w.findAll('[data-testid="guide-row"]')[0].trigger('click')
    await w.find('[data-testid="guide-delete-file"]').trigger('click')
    await flushPromises()
    expect(ui.confirm).toHaveBeenCalled()
    expect(api.deleteGuideFile).toHaveBeenCalledWith('user')
    expect(del.notify).toHaveBeenCalled()
  })

  it('не удаляет файл при отмене подтверждения', async () => {
    ui.confirm.mockResolvedValueOnce(false)
    const w = mountCmp()
    await flushPromises()
    await w.findAll('[data-testid="guide-row"]')[0].trigger('click')
    await w.find('[data-testid="guide-delete-file"]').trigger('click')
    await flushPromises()
    expect(api.deleteGuideFile).not.toHaveBeenCalled()
  })

  it('save при ошибке: инлайн-ошибка + уведомление type:error', async () => {
    api.updateGuideSection.mockRejectedValue(new Error('Бэкенд упал'))
    const w = mountCmp()
    await flushPromises()
    await w.findAll('[data-testid="guide-row"]')[2].trigger('click')
    await w.find('[data-testid="guide-lead"]').setValue('Новый лид')
    await w.find('[data-testid="guide-save"]').trigger('click')
    await flushPromises()
    expect(w.find('.form-error').text()).toContain('Бэкенд упал')
    expect(del.notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
  })

  it('не переключает раздел при отмене dirty-подтверждения', async () => {
    const w = mountCmp()
    await flushPromises()
    await w.findAll('[data-testid="guide-row"]')[0].trigger('click')
    await w.find('[data-testid="guide-lead"]').setValue('изменённый лид')
    confirmIfAnyDirty.mockResolvedValueOnce(false)
    await w.findAll('[data-testid="guide-row"]')[2].trigger('click')
    await flushPromises()
    // остались на разделе пользователя: правки не сброшены к выбранному разделу
    expect(w.find('[data-testid="guide-lead"]').element.value).toBe('изменённый лид')
    expect(w.find('.details-title').text()).toBe('Пользователь')
  })
})
