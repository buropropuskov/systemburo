import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve([])),
}));
import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import TableConstructor from '../TableConstructor.vue';

const shellStub = { template: '<div><slot /></div>' };

/**
 * Монтирует конструктор с застабленными детьми и spy на уведомления.
 * @returns {{ wrapper: import('@vue/test-utils').VueWrapper, notify: import('vitest').Mock }}
 */
function mountC() {
  setActivePinia(createPinia());
  usePermissionsStore().mode = 'super';
  const notify = vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {});
  const wrapper = mount(TableConstructor, {
    global: {
      stubs: {
        AdminPageShell: shellStub,
        SearchComponent: true,
        RefreshButton: true,
        BaseDropdown: true,
        TextConstructor: true,
        WorkScheduleTab: true,
        SystemTableColumnsTab: true,
        SystemTableAppearanceTab: true,
        TableConstructorCreateModal: true,
        TableConstructorPhotoSection: true,
        SystemTableHistoryModal: true,
        ConfirmationModal: true,
      },
      mocks: { $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } },
    },
  });
  return { wrapper, notify };
}

// Ответ apiRequest на ошибку: wrapJsonUnwrap на !success отдаёт { message: body.error }.
function errResponse(message) {
  return { ok: false, json: vi.fn().mockResolvedValue({ message }) };
}

describe('TableConstructor — уведомления об ошибках операций с таблицей', () => {
  beforeEach(() => {
    apiRequest.mockClear();
    apiRequest.mockResolvedValue([]);
  });

  it('ошибка удаления привязанной таблицы: показывает чистое сообщение бэка, без сырого JSON', async () => {
    const { wrapper, notify } = mountC();
    const msg = 'Невозможно удалить таблицу, так как она привязана к: организациям (2)';
    apiRequest.mockResolvedValueOnce(errResponse(msg));
    notify.mockClear();

    wrapper.vm.deleteConfirmTable = { table: { id: 7, display_name: 'Грузовые' } };
    await wrapper.vm.performDeleteTable();

    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ prefix: 'Не удалось архивировать: ', bold: msg, type: 'error' }),
    );
    // Регресс: в bold не должно быть артефактов сырого envelope ({ }, имена полей).
    const arg = notify.mock.calls.at(-1)[0];
    expect(arg.bold).not.toContain('{');
    expect(arg.bold).not.toContain('success');
  });

  it('ошибка восстановления читается тем же путём (message, а не сырое тело)', async () => {
    const { wrapper, notify } = mountC();
    apiRequest.mockResolvedValueOnce(errResponse('Таблица не в архиве'));
    notify.mockClear();

    await wrapper.vm.restoreTable({ table: { id: 7, display_name: 'Грузовые' } });

    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ prefix: 'Ошибка восстановления: ', bold: 'Таблица не в архиве', type: 'error' }),
    );
  });

  it('нечитаемое тело ответа не роняет уведомление - дефолтное сообщение', async () => {
    const { wrapper, notify } = mountC();
    apiRequest.mockResolvedValueOnce({ ok: false, json: vi.fn().mockRejectedValue(new Error('not json')) });
    notify.mockClear();

    wrapper.vm.deleteConfirmTable = { table: { id: 7, display_name: 'Грузовые' } };
    await wrapper.vm.performDeleteTable();

    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ prefix: 'Не удалось архивировать: ', bold: 'неизвестная ошибка', type: 'error' }),
    );
  });
});
