import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import Modal from '../ApplicationDraftFilesModal.vue';

const { fetchMock, deleteMock, notifyMock } = vi.hoisted(() => ({ fetchMock: vi.fn(), deleteMock: vi.fn(), notifyMock: vi.fn() }));
vi.mock('@/api/applicationFiles', () => ({ fetchApplicationDraftFiles: fetchMock, deleteApplicationDraftFile: deleteMock }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify: notifyMock }) }));
vi.mock('@/api/client', () => ({ API_BASE_URL: '/api' }));

function snapshot() { return { items: [{ id: 7, file_name: 'example.pdf', file_size: 1024, created_at: '2040-01-02T12:00:00Z' }], usage: { saved_count: 1, saved_bytes: 1024, reserved_count: 0, reserved_bytes: 0, cleanup_count: 0 }, limits: { max_count: 50, max_bytes: 209715200 }, meta: { total: 1, page: 1, per_page: 20 } }; }
let wrapper;
beforeEach(() => { vi.clearAllMocks(); fetchMock.mockImplementation(async () => snapshot()); deleteMock.mockResolvedValue(undefined); });
afterEach(() => { wrapper?.unmount(); document.body.innerHTML = ''; });
async function open() { wrapper = mount(Modal, { props: { show: true }, attachTo: document.body }); await flushPromises(); return document.body; }

describe('ApplicationDraftFilesModal', () => {
  it('shows own metadata and requests a bounded page', async () => {
    const body = await open();
    expect(fetchMock).toHaveBeenCalledWith(1, 20);
    expect(body.textContent).toContain('example.pdf');
    expect(body.textContent).toContain('Файлы-черновики');
    expect(body.textContent).toContain('из 50 файлов');
    expect(body.querySelector('[role=dialog]').style.getPropertyValue('--base-modal-radius')).toBe('30px');
  });
  it('requires selecting a concrete file before deleting and refreshes usage', async () => {
    const body = await open();
    expect(deleteMock).not.toHaveBeenCalled();
    body.querySelector('.draft-remove').click(); await flushPromises();
    expect(deleteMock).not.toHaveBeenCalled();
    expect(body.textContent).toContain('Он может использоваться в другой вкладке');
    body.querySelector('.draft-confirm .lk-button--danger').click(); await flushPromises();
    expect(deleteMock).toHaveBeenCalledWith(7);
    expect(wrapper.emitted('deleted')).toEqual([[7]]);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
  it('shows an explicit retry after a list failure', async () => {
    fetchMock.mockRejectedValueOnce(new Error('Список недоступен'));
    const body = await open();
    expect(body.textContent).toContain('Не удалось загрузить список');
    body.querySelector('.draft-empty button').click(); await flushPromises();
    expect(body.textContent).toContain('example.pdf');
  });
  it('accounts for ongoing cleanup and reports a full quota', async () => {
    const data = snapshot(); data.usage = { saved_count: 49, saved_bytes: 1024, reserved_count: 1, reserved_bytes: 1024, cleanup_count: 1 };
    fetchMock.mockResolvedValue(data);
    const body = await open();
    expect(body.textContent).toContain('Достигнут лимит файлов-черновиков');
    expect(body.textContent).toContain('Ожидают очистки: 1');
    expect(body.textContent).not.toContain('Есть выполняющиеся загрузки');
  });
});
