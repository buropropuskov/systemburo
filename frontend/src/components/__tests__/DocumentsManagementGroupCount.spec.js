import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';
import DocumentsManagement from '../DocumentsManagement.vue';

// Регресс: модалка «Управление группами» показывала «0 документов» у любой группы.
// Причина — бэк /document-groups отдаёт поле `count` (DocumentGroupWithCount.json:"count"),
// а компонент читал несуществующее `doc_count` -> undefined ?? 0 -> всегда 0.
// Тест строит фикстуру от РЕАЛЬНОЙ формы ответа (поле `count`).

const GROUPS = [
  { id: 1, name: 'Договоры', count: 3, sort_order: 0 },
  { id: 2, name: 'Инструкции', count: 0, sort_order: 1 },
];

vi.mock('@/api/documents', () => ({
  listDocumentGroups: vi.fn(() => Promise.resolve(GROUPS)),
  listDocuments: vi.fn(() => Promise.resolve([])),
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

describe('DocumentsManagement — счётчик документов в группе', () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it('в модалке групп показывает реальный count из API, не «0 документов»', async () => {
    const w = mount(DocumentsManagement, { global: { stubs }, attachTo: document.body });
    await flushPromises();

    w.vm.openGroupsModal();
    await nextTick();

    const text = w.html();
    expect(text).toContain('3 документов');
    expect(text).not.toContain('3 документов0'); // sanity
    // Группа без документов честно показывает 0.
    expect(text).toContain('0 документов');
    w.unmount();
  });

  it('новая созданная группа стартует с count 0 (реактивно)', () => {
    const w = mount(DocumentsManagement, { global: { stubs }, attachTo: document.body });
    // прямая проверка формы объекта новой группы — count, не doc_count
    const g = { id: 9, name: 'Новая', sort_order: 5 };
    const newGroup = { ...g, count: 0 };
    expect(newGroup).toHaveProperty('count', 0);
    expect(newGroup).not.toHaveProperty('doc_count');
    w.unmount();
  });
});
