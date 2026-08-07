import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  uploadApplicationFiles,
  fetchApplicationFiles,
  deleteApplicationDraftFile,
} from '@/api/applicationFiles';

// Тесты на контракт с client.js, а не на компоненты: apiRequest разворачивает
// конверт {success, data}, поэтому json() отдаёт уже сами данные, а при отказе -
// {message}. Компонентные тесты этого не ловят - они мокают весь модуль api, и
// ровно на этом список файлов оставался пустым при успешной загрузке.

const { apiRequestMock } = vi.hoisted(() => ({ apiRequestMock: vi.fn() }));

vi.mock('@/api/client', () => ({ apiRequest: apiRequestMock }));
vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn(() => ({ token: 'test-token' })) }));

/** Ответ в том виде, в каком его отдаёт обёрнутый apiRequest. */
function unwrapped(ok, body) {
  return { ok, json: async () => body };
}

describe('api/applicationFiles', () => {
    beforeEach(() => vi.clearAllMocks());

    it('возвращает загруженные файлы, а не пустой список', async () => {
        const saved = [{ id: 7, file_name: 'разрешение.png', file_size: 84, mime_type: 'image/png' }];
        apiRequestMock.mockResolvedValue(unwrapped(true, saved));

        const files = await uploadApplicationFiles([new File(['x'], 'разрешение.png')]);

        expect(files).toEqual(saved);
        const [path, options] = apiRequestMock.mock.calls[0];
        expect(path).toBe('/applications/files');
        expect(options.body).toBeInstanceOf(FormData);
        // Заголовки пустые: boundary проставляет браузер сам.
        expect(options.headers).toEqual({});
    });

    it('поднимает сообщение сервера при отказе', async () => {
        apiRequestMock.mockResolvedValue(unwrapped(false, { message: 'Файл слишком большой' }));

        await expect(uploadApplicationFiles([new File(['x'], 'огромный.png')]))
            .rejects.toThrow('Файл слишком большой');
    });

    it('возвращает список файлов заявки', async () => {
        const rows = [{ id: 3, file_name: 'разрешение.pdf', file_size: 2048, mime_type: 'application/pdf' }];
        apiRequestMock.mockResolvedValue(unwrapped(true, rows));

        await expect(fetchApplicationFiles(42)).resolves.toEqual(rows);
        expect(apiRequestMock).toHaveBeenCalledWith('/applications/42/files');
    });

    it('пустой ответ не превращается в undefined', async () => {
        apiRequestMock.mockResolvedValue(unwrapped(true, []));

        await expect(fetchApplicationFiles(42)).resolves.toEqual([]);
    });

    it('сообщает причину, когда черновик убрать не удалось', async () => {
        apiRequestMock.mockResolvedValue(unwrapped(false, { message: 'Файл не найден' }));

        await expect(deleteApplicationDraftFile(7)).rejects.toThrow('Файл не найден');
    });
});
