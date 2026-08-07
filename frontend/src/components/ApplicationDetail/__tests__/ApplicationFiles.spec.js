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

// Плитка красится в тон своего формата - взгляд отличает pdf от снимка, не читая имя.
describe('ApplicationFiles: цвет по типу', () => {
    beforeEach(() => vi.clearAllMocks());

    it('плитка pdf получает класс своего формата', async () => {
        fetchMock.mockResolvedValue([file]);
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42 } });
        await flushPromises();

        expect(wrapper.find('[data-testid="application-file-item"]').classes())
            .toContain('app-files-strip__tile--pdf');
    });

    it('снимок и таблица красятся по-разному', async () => {
        fetchMock.mockResolvedValue([
            { id: 1, file_name: 'скан.png', file_size: 10, mime_type: 'image/png' },
            { id: 2, file_name: 'смета.xlsx', file_size: 20, mime_type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' },
        ]);
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42 } });
        await flushPromises();

        const tiles = wrapper.findAll('[data-testid="application-file-item"]');
        expect(tiles[0].classes()).toContain('app-files-strip__tile--image');
        expect(tiles[1].classes()).toContain('app-files-strip__tile--sheet');
    });
});

// Формат определяется по расширению имени, а не по типу из базы: docx, xlsx и pptx
// неразличимы по сигнатуре, и у файлов, загруженных до уточнения типа, в базе
// лежит docx независимо от того, чем файл был на самом деле.
describe('ApplicationFiles: формат по имени', () => {
    beforeEach(() => vi.clearAllMocks());

    it('таблица со старым типом docx всё равно зелёная', async () => {
        fetchMock.mockResolvedValue([{
            id: 5,
            file_name: 'смета.xlsx',
            file_size: 100,
            mime_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        }]);
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42 } });
        await flushPromises();

        expect(wrapper.find('[data-testid="application-file-item"]').classes())
            .toContain('app-files-strip__tile--sheet');
        expect(wrapper.find('.app-files-strip__ext').text()).toBe('xlsx');
    });

    it('презентация получает свой цвет', async () => {
        fetchMock.mockResolvedValue([{ id: 6, file_name: 'доклад.pptx', file_size: 100, mime_type: 'application/octet-stream' }]);
        const wrapper = mount(ApplicationFiles, { props: { applicationId: 42 } });
        await flushPromises();

        expect(wrapper.find('[data-testid="application-file-item"]').classes())
            .toContain('app-files-strip__tile--slides');
    });
});
