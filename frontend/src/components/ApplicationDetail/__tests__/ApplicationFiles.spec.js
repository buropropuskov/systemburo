import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import ApplicationFiles from '../ApplicationFiles.vue';

// Плитки вложений над текстом письма, как в почтовом клиенте. Скачивание идёт через
// Blob с токеном, поэтому плитка - кнопка, а не ссылка: href не несёт авторизацию.

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

        await wrapper.find('[data-testid="application-file-item"]').trigger('click');
        await flushPromises();

        expect(downloadMock).toHaveBeenCalledWith(42, 3, 'разрешение.pdf');
    });

    it('прячет удаление у того, кто не администратор', async () => {
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42 } });
        await flushPromises();

        expect(wrapper.find('.app-files-strip__remove').exists()).toBe(false);
    });

    it('убирает файл, когда удаление разрешено', async () => {
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42, canRemove: true } });
        await flushPromises();

        await wrapper.find('.app-files-strip__remove').trigger('click');
        await flushPromises();

        expect(deleteMock).toHaveBeenCalledWith(42, 3);
        expect(wrapper.find('[data-testid="application-file-item"]').exists()).toBe(false);
    });

    it('не занимает место над сообщением, когда файлов нет', async () => {
        fetchMock.mockResolvedValueOnce([]);
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42 } });
        await flushPromises();

        expect(wrapper.find('[data-testid="application-files"]').exists()).toBe(false);
    });
});

// Плитка показывает формат и размер: в шапке письма важно с одного взгляда понять,
// что приложено, не открывая файл.
describe('ApplicationFiles: вид плитки', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        downloadMock.mockResolvedValue(undefined);
    });

    it('ставит на плашку расширение файла', async () => {
        fetchMock.mockResolvedValue([file]);
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42 } });
        await flushPromises();

        expect(wrapper.find('.app-files-strip__ext').text()).toBe('pdf');
        expect(wrapper.find('.app-files-strip__ext').classes()).toContain('app-files-strip__ext--pdf');
    });

    it('имя без расширения не превращается в мусорную плашку', async () => {
        fetchMock.mockResolvedValue([{ id: 9, file_name: 'скан', file_size: 100, mime_type: 'image/png' }]);
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42 } });
        await flushPromises();

        expect(wrapper.find('.app-files-strip__ext').text()).toBe('файл');
        expect(wrapper.find('.app-files-strip__ext').classes()).toContain('app-files-strip__ext--image');
    });
});
