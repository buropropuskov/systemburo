import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

/**
 * Вход в массовый ввод и кнопка «Скачать пустой бланк» показываются только там, где
 * бланк действительно настроен: у типа вложения есть шаблон со списочными привязками.
 *
 * Без этой проверки форма звала в импорт всегда, а эндпоинт отвечал на клик отказом
 * 404 «Шаблон бланка не настроен» - человек упирался в тупик уже после действия.
 * Признак приходит с сервера полем `blank_import_ready` в списке типов вложений.
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
  createExtendedTimeoutSignal: vi.fn(() => undefined),
}));

const права = { list: ['action.import.list'] };
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission: (k) => права.list.includes(k) }),
}));

import CreateApplication from '../CreateApplication.vue';

function mountForm() {
  setActivePinia(createPinia());
  return shallowMount(CreateApplication, {
    global: { mocks: { $route: { query: {} }, $router: { replace: vi.fn(), push: vi.fn() } } },
  });
}

const бланк = (extra) => ({ local_id: 'a1', attachment_type: 'people', display_name: 'Люди', ...extra });

describe('CreateApplication — импорт предлагается только с готовым бланком', () => {
  beforeEach(() => {
    права.list = ['action.import.list'];
    setActivePinia(createPinia());
  });

  it('готовый бланк открывает импорт', async () => {
    const w = mountForm();
    await w.setData({ selectedAttachment: бланк({ blank_import_ready: true }) });
    expect(w.vm.canImportList).toBe(true);
    w.unmount();
  });

  it('бланк без размеченного списка импорт не предлагает', async () => {
    const w = mountForm();
    await w.setData({ selectedAttachment: бланк({ blank_import_ready: false }) });
    expect(w.vm.canImportList, 'иначе клик упрётся в 404 «Шаблон бланка не настроен»').toBe(false);
    w.unmount();
  });

  it('старый черновик без признака импорт не теряет', async () => {
    // Признак завели позже: у сохранённого черновика поля просто нет, и молчание не
    // должно читаться как запрет - отказ даёт только явный false.
    const w = mountForm();
    await w.setData({ selectedAttachment: бланк({}) });
    expect(w.vm.canImportList).toBe(true);
    w.unmount();
  });

  it('без права импорта готовность бланка ничего не открывает', async () => {
    права.list = [];
    const w = mountForm();
    await w.setData({ selectedAttachment: бланк({ blank_import_ready: true }) });
    expect(w.vm.canImportList).toBe(false);
    w.unmount();
  });
});
