import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import ApplicationFilesUpload from '../ApplicationFilesUpload.vue';

// Файлы уходят на сервер сразу при выборе и живут черновиками до подачи, поэтому
// наверх компонент отдаёт id, а не объекты File: к моменту отправки формы файлы
// уже на диске сервера, и подача только связывает их с заявкой.

const { notifyMock, uploadMock, deleteMock } = vi.hoisted(() => ({
    notifyMock: vi.fn(),
    uploadMock: vi.fn(),
    deleteMock: vi.fn(),
}));

vi.mock('@/api/applicationFiles', () => ({
    uploadApplicationFiles: uploadMock,
    deleteApplicationDraftFile: deleteMock,
}));
vi.mock('@/stores/deletions', () => ({
    useDeletionsStore: vi.fn(() => ({ notify: notifyMock, enqueue: vi.fn() })),
}));

function fakeFile(name) {
    return new File(['payload'], name, { type: 'image/png' });
}

/** Подменяет FileList у скрытого input: jsdom не даёт присвоить files напрямую. */
async function pick(wrapper, files) {
    const input = wrapper.find('input[type="file"]');
    Object.defineProperty(input.element, 'files', { value: files, configurable: true });
    await input.trigger('change');
    await flushPromises();
}

describe('ApplicationFilesUpload', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        uploadMock.mockResolvedValue([{ id: 7, file_name: 'разрешение.png', file_size: 1024, mime_type: 'image/png' }]);
        deleteMock.mockResolvedValue(undefined);
    });

    it('загружает выбранный файл и отдаёт наверх его id', async () => {
        const wrapper = mount(ApplicationFilesUpload, { props: { modelValue: [] } });

        await pick(wrapper, [fakeFile('разрешение.png')]);

        expect(uploadMock).toHaveBeenCalledTimes(1);
        expect(wrapper.emitted('update:modelValue').at(-1)).toEqual([[7]]);
        expect(wrapper.text()).toContain('разрешение.png');
        // Счётчик уехал в подсказку кнопки: полоса живёт внутри письма и лишних
        // подписей не несёт.
        expect(wrapper.find('[data-testid="app-files-add"]').attributes('title')).toContain('1 из 10');
    });

    it('не отправляет файлы сверх лимита и объясняет отказ', async () => {
        const wrapper = mount(ApplicationFilesUpload, { props: { modelValue: [], maxCount: 1 } });

        await pick(wrapper, [fakeFile('первый.png'), fakeFile('второй.png')]);

        expect(uploadMock).not.toHaveBeenCalled();
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    });

    it('убирает файл и с сервера, и из списка id', async () => {
        const wrapper = mount(ApplicationFilesUpload, { props: { modelValue: [] } });
        await pick(wrapper, [fakeFile('разрешение.png')]);

        await wrapper.find('.app-files__remove').trigger('click');
        await flushPromises();

        expect(deleteMock).toHaveBeenCalledWith(7);
        expect(wrapper.emitted('update:modelValue').at(-1)).toEqual([[]]);
        expect(wrapper.text()).not.toContain('разрешение.png');
    });

    it('сообщает об ошибке загрузки, не добавляя файл в список', async () => {
        uploadMock.mockRejectedValueOnce(new Error('Файл слишком большой'));
        const wrapper = mount(ApplicationFilesUpload, { props: { modelValue: [] } });

        await pick(wrapper, [fakeFile('огромный.png')]);

        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({ bold: 'Файл слишком большой', type: 'error' }));
        expect(wrapper.find('[data-testid="app-files-item"]').exists()).toBe(false);
    });
});
