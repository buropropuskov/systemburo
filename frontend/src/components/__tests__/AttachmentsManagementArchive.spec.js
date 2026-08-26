import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import AttachmentsManagement from '@/components/AttachmentsManagement.vue'
import { useDeletionsStore } from '@/stores/deletions'

/**
 * Тумблер «Автосохранение в файловый архив» в карточке типа вложения (#1615, срез
 * C5): гейт по глобальному рубильнику архива и по наличию активного Excel-бланка,
 * а также фактическая отправка auto_export в PUT при сохранении.
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue({}) }),
}))

const attachmentsApi = vi.hoisted(() => ({
  listAllAttachments: vi.fn(),
  createAttachment: vi.fn(),
  updateAttachment: vi.fn(),
  archiveAttachment: vi.fn(),
  restoreAttachment: vi.fn(),
}))
vi.mock('@/api/attachments', () => attachmentsApi)

const templatesApi = vi.hoisted(() => ({ getTemplate: vi.fn() }))
vi.mock('@/api/attachment-templates', () => templatesApi)

const fileArchiveApi = vi.hoisted(() => ({ getArchiveSettings: vi.fn() }))
vi.mock('@/api/fileArchive', () => fileArchiveApi)

function seedAttachments() {
  return [
    {
      id: 1, display_name: 'Автозаявка', name: 'avtozayavka', title: 'АВТО',
      attachment_type: 'cars', instruction: '', is_active: true, auto_export: true,
    },
  ]
}

function mountCmp() {
  attachmentsApi.listAllAttachments.mockResolvedValue(seedAttachments())
  return mount(AttachmentsManagement, {
    global: {
      stubs: {
        Teleport: true,
        ConfirmationModal: true,
        TextConstructor: true,
        AttachmentTemplateEditor: true,
        UniqueAttachmentHistoryModal: true,
        AttachmentFieldsModal: true,
      },
    },
  })
}

const toggle = w => w.find('[data-testid="attachment-auto-export"] input[type="checkbox"]')
const hintAnchor = w => w.find('[data-testid="attachment-auto-export-hint"]')

describe('AttachmentsManagement — тумблер файлового архива', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {})
  })
  afterEach(() => vi.clearAllMocks())

  it('архив включён и у типа есть активный бланк - тумблер доступен', async () => {
    fileArchiveApi.getArchiveSettings.mockResolvedValue({ enabled: true })
    templatesApi.getTemplate.mockResolvedValue({ file_path: '/uploads/templates/a.xlsx' })
    const w = mountCmp()
    await flushPromises()

    await w.find('[data-testid="attachment-row"]').trigger('click')
    await flushPromises()

    expect(toggle(w).attributes('disabled')).toBeUndefined()
  })

  it('архив выключен глобально - тумблер заблокирован с подсказкой', async () => {
    fileArchiveApi.getArchiveSettings.mockResolvedValue({ enabled: false })
    templatesApi.getTemplate.mockResolvedValue({ file_path: '/uploads/templates/a.xlsx' })
    const w = mountCmp()
    await flushPromises()

    await w.find('[data-testid="attachment-row"]').trigger('click')
    await flushPromises()

    expect(toggle(w).attributes('disabled')).toBeDefined()
    expect(hintAnchor(w).attributes('data-hint')).toContain('выключен')
  })

  it('у типа нет активного бланка - тумблер заблокирован с подсказкой', async () => {
    fileArchiveApi.getArchiveSettings.mockResolvedValue({ enabled: true })
    templatesApi.getTemplate.mockResolvedValue({ message: 'Шаблон не настроен' })
    const w = mountCmp()
    await flushPromises()

    await w.find('[data-testid="attachment-row"]').trigger('click')
    await flushPromises()

    expect(toggle(w).attributes('disabled')).toBeDefined()
    expect(hintAnchor(w).attributes('data-hint')).toContain('бланка')
  })

  // КРИТИЧНО (#1615): PUT - полная замена полей, тумблер обязан уходить в теле
  // запроса при каждом сохранении формы деталей, иначе правка молча теряется.
  it('переключение тумблера делает форму dirty и уходит в PUT при сохранении', async () => {
    fileArchiveApi.getArchiveSettings.mockResolvedValue({ enabled: true })
    templatesApi.getTemplate.mockResolvedValue({ file_path: '/uploads/templates/a.xlsx' })
    attachmentsApi.updateAttachment.mockResolvedValue({})
    const w = mountCmp()
    await flushPromises()

    await w.find('[data-testid="attachment-row"]').trigger('click')
    await flushPromises()

    expect(w.find('[data-testid="attachment-save"]').attributes('disabled')).toBeDefined()

    await toggle(w).setValue(false)
    expect(w.find('[data-testid="attachment-save"]').attributes('disabled')).toBeUndefined()

    await w.find('[data-testid="attachment-save"]').trigger('click')
    await flushPromises()

    expect(attachmentsApi.updateAttachment).toHaveBeenCalledWith(1, expect.objectContaining({ autoExport: false }))
  })
})
