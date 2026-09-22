import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import DocumentsManagement from '../DocumentsManagement.vue';

// Замок на петлю обновлений в панели правки: локальная копия формы синхронизировалась
// с пропсом через пару deep-watch, каждый шаг отдавал наружу новый объект, и ввод
// одного символа гонял update:form по кругу до отказа вкладки (Chrome падал на
// /admin/documents). Считаем обновления формы: на один ввод их единицы, не сотни.

const DOCS = [
  {
    id: 7,
    title: 'Автозаявка',
    description: '',
    comment: '',
    file_name: 'auto.xlsx',
    file_ext: '.xlsx',
    file_size: 430080,
    group_id: null,
    published_at: '2026-06-20T00:00:00Z',
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

describe('DocumentsManagement — правка документа не зацикливает обновления', () => {
  let warn;

  beforeEach(() => {
    vi.clearAllMocks();
    warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
  });
  afterEach(() => { warn.mockRestore(); });

  it('ввод пояснения обновляет форму считанные разы и не уходит в рекурсию', async () => {
    const w = mount(DocumentsManagement, { global: { stubs }, attachTo: document.body });
    await flushPromises();
    await w.find('.docs-row').trigger('click');

    let updates = 0;
    w.vm.$watch('editForm', () => { updates += 1; }, { deep: true });

    await w.find('[data-testid="document-comment-field"]').setValue('Заполнять как в накладной');
    await flushPromises();

    expect(w.vm.editForm.comment).toBe('Заполнять как в накладной');
    expect(updates).toBeLessThan(5);
    expect(warn.mock.calls.flat().join(' ')).not.toContain('Maximum recursive updates');
    w.unmount();
  });

  it('выбор другого поля не трёт уже введённое пояснение', async () => {
    const w = mount(DocumentsManagement, { global: { stubs }, attachTo: document.body });
    await flushPromises();
    await w.find('.docs-row').trigger('click');

    await w.find('[data-testid="document-comment-field"]').setValue('Пояснение бюро');
    await w.find('.lk-input[type="text"]').setValue('Автозаявка 2026');
    await flushPromises();

    expect(w.vm.editForm.comment).toBe('Пояснение бюро');
    expect(w.vm.editForm.title).toBe('Автозаявка 2026');
    w.unmount();
  });
});
