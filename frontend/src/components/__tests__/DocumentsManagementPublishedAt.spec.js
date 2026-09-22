import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import DocumentsManagement from '../DocumentsManagement.vue';
import { updateDocument } from '@/api/documents';

// Замок на формат даты публикации: input[type=date] отдаёт YYYY-MM-DD, бэкенд
// принимает только RFC3339 и отвечал «Неверный формат даты публикации» на любое
// сохранение документа с заполненной датой. Смещение московское, иначе дата
// уезжает на день назад: интерфейс печатает её по Москве.

const DOCS = [
  {
    id: 8,
    title: 'Заявка на ввоз',
    description: '',
    comment: '',
    file_name: 'vvoz.xlsx',
    file_ext: '.xlsx',
    file_size: 30720,
    group_id: 2,
    published_at: '2026-06-19T21:00:00Z',
    created_at: '2026-06-18T10:00:00Z',
    is_visible: true,
  },
];

vi.mock('@/api/documents', () => ({
  listDocumentGroups: vi.fn(() => Promise.resolve([{ id: 2, name: 'Бланки заявок' }])),
  listDocuments: vi.fn(() => Promise.resolve(DOCS)),
  createDocumentGroup: vi.fn(),
  renameDocumentGroup: vi.fn(),
  deleteDocumentGroup: vi.fn(),
  reorderDocumentGroups: vi.fn(),
  uploadDocument: vi.fn(),
  updateDocument: vi.fn(() => Promise.resolve({})),
  replaceDocumentFile: vi.fn(),
  deleteDocument: vi.fn(),
  reorderDocuments: vi.fn(),
  downloadDocument: vi.fn(),
}));
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: vi.fn(() => ({ notify: vi.fn(), enqueue: vi.fn() })),
}));
vi.mock('@/composables/useOverlayClose', () => ({
  useOverlayClose: vi.fn(() => ({ onOverlayMousedown: vi.fn(), onOverlayMouseup: vi.fn() })),
}));

const stubs = {
  RefreshButton: true, ConfirmationModal: true, BaseDropdown: true,
  LoaderSpinner: true, FileTypeIcon: true, Teleport: true,
};

describe('DocumentsManagement — дата публикации при сохранении', () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it('уходит на бэкенд в RFC3339 с московским смещением, а не голой датой', async () => {
    const w = mount(DocumentsManagement, { global: { stubs }, attachTo: document.body });
    await flushPromises();
    await w.find('.docs-row').trigger('click');

    await w.find('[data-testid="document-comment-field"]').setValue('Пояснение бюро');
    await w.find('.details-section .lk-button--primary').trigger('click');
    await flushPromises();

    const [, payload] = updateDocument.mock.calls[0];
    expect(payload.published_at).toBe('2026-06-20T00:00:00+03:00');
    expect(payload.comment).toBe('Пояснение бюро');
    w.unmount();
  });

  it('в поле показывается московская дата, а не срез UTC-строки', async () => {
    const w = mount(DocumentsManagement, { global: { stubs }, attachTo: document.body });
    await flushPromises();
    await w.find('.docs-row').trigger('click');

    expect(w.find('.details-section input[type="date"]').element.value).toBe('2026-06-20');
    w.unmount();
  });

  it('пустая дата уходит как null, а не как пустая строка', async () => {
    const w = mount(DocumentsManagement, { global: { stubs }, attachTo: document.body });
    await flushPromises();
    await w.find('.docs-row').trigger('click');

    await w.find('.details-section input[type="date"]').setValue('');
    await w.find('.details-section .lk-button--primary').trigger('click');
    await flushPromises();

    const [, payload] = updateDocument.mock.calls[0];
    expect(payload.published_at).toBeNull();
    w.unmount();
  });
});
