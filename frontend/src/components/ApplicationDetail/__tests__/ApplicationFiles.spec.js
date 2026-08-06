import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import ApplicationFiles from '../ApplicationFiles.vue';

// Блок файлов в детали заявки. Скачивание идёт через Blob с токеном, поэтому имя
// файла кликабельно, а не оформлено ссылкой: обычный href не несёт авторизацию.

const { notifyMock, fetchMock, downloadMock, deleteMock } = vi.hoisted(() => ({
    notifyMock: vi.fn(),
    fetchMock: vi.fn(),
    downloadMock: vi.fn(),
    deleteMock: vi.fn(),
}));

vi.mock('@/api/applicationFiles', () => ({
    fetchApplicationFiles: fetchMock,
    downloadApplicationFile: downloadMock,
    deleteApplicationFile: deleteMock,
}));
vi.mock('@/stores/deletions', () => ({
    useDeletionsStore: vi.fn(() => ({ notify: notifyMock, enqueue: vi.fn() })),
}));

const file = { id: 3, file_name: 'разрешение.pdf', file_size: 2048, mime_type: 'application/pdf' };

describe('ApplicationFiles', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        fetchMock.mockResolvedValue([file]);
        downloadMock.mockResolvedValue(undefined);
        deleteMock.mockResolvedValue(undefined);
    });

    it('показывает файлы заявки с размером', async () => {
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42 } });
        await flushPromises();

        expect(fetchMock).toHaveBeenCalledWith(42);
        expect(wrapper.text()).toContain('разрешение.pdf');
        expect(wrapper.text()).toContain('2 КБ');
    });

    it('скачивает файл по клику на имя', async () => {
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42 } });
        await flushPromises();

        await wrapper.find('.app-files-view__name').trigger('click');
        await flushPromises();

        expect(downloadMock).toHaveBeenCalledWith(42, 3, 'разрешение.pdf');
    });

    it('прячет удаление, когда права на него нет', async () => {
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42 } });
        await flushPromises();

        expect(wrapper.find('.app-files-view__remove').exists()).toBe(false);
    });

    it('убирает файл, когда удаление разрешено', async () => {
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42, canRemove: true } });
        await flushPromises();

        await wrapper.find('.app-files-view__remove').trigger('click');
        await flushPromises();

        expect(deleteMock).toHaveBeenCalledWith(42, 3);
        expect(wrapper.find('[data-testid="application-file-item"]').exists()).toBe(false);
    });

    it('не занимает место в карточке, когда файлов нет', async () => {
        fetchMock.mockResolvedValueOnce([]);
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42 } });
        await flushPromises();

        expect(wrapper.find('[data-testid="application-files"]').exists()).toBe(false);
    });
});
