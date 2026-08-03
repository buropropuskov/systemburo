import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import DocumentsManagement from '../DocumentsManagement.vue';

// Замок на единый формат размеров: снятая локальная копия formatFileSize не знала
// гигабайтов и печатала «1228.8 МБ» вместо «1.2 ГБ». Формат тот же, что у CLI
// server archive (humanBytes) и у вкладки «Файловый архив».

const DOCS = [
  {
    id: 7,
    title: 'Договор',
    file_name: 'dogovor.pdf',
    file_ext: '.pdf',
    file_size: 1288490188,
    group_id: null,
    created_at: '2026-06-18T10:00:00Z',
    is_visible: true,
  },
];

vi.mock('@/api/documents', () => ({
  listDocumentGroups: vi.fn(() => Promise.resolve([])),
  listDocuments: vi.fn(() => Promise.resolve(DOCS)),
  createDocumentGroup: vi.fn(),
  renameDocumentGroup: vi.fn(),
  deleteDocumentGroup: vi.fn(),
  reorderDocumentGroups: vi.fn(),
  uploadDocument: vi.fn(),
  updateDocument: vi.fn(),
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

describe('DocumentsManagement — размер файла в панели деталей', () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it('гигабайтный документ показывает «1.2 ГБ», а не сотни мегабайт', async () => {
    const w = mount(DocumentsManagement, { global: { stubs }, attachTo: document.body });
    await flushPromises();

    await w.find('.docs-row').trigger('click');
    expect(w.find('.doc-detail-meta').text()).toContain('1.2 ГБ');
    w.unmount();
  });
});
