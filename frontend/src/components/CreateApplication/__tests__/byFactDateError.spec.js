import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import { BY_FACT_ONE_DAY_HINT } from '@/utils/byFactVehicle';

/**
 * Период заявки с машиной «По факту» подсвечивается ошибкой ещё до того, как
 * машину положат в список (#2320).
 *
 * Замок смонтированный, а не текстовый: предыдущая проверка сверяла исходник и
 * пропустила нерабочий код - флаг byFactPending использовался в шаблоне и в
 * computed, но не был объявлен в data. Vue молча отдаёт undefined, подсветка не
 * появлялась, а тест по исходнику оставался зелёным.
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
  createExtendedTimeoutSignal: vi.fn(() => undefined),
}));

import CreateApplication from '../CreateApplication.vue';

function mountForm() {
  setActivePinia(createPinia());
  return shallowMount(CreateApplication, {
    global: { mocks: { $route: { query: {} }, $router: { replace: vi.fn(), push: vi.fn() } } },
  });
}

describe('CreateApplication — ошибка периода у машины «По факту»', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('флаг включённого тумблера существует и по умолчанию выключен', () => {
    const w = mountForm();
    expect(w.vm.byFactPending, 'byFactPending обязан быть объявлен в data').toBe(false);
    w.unmount();
  });

  it('включённый тумблер подсвечивает обе даты, пока период длиннее дня', async () => {
    const w = mountForm();
    const key = w.vm.attachmentKey({ local_id: 'a1', attachment_type: 'cars' });
    await w.setData({
      attachments: [{ local_id: 'a1', attachment_type: 'cars', display_name: 'Авто' }],
      selectedAttachment: { local_id: 'a1', attachment_type: 'cars', display_name: 'Авто' },
      attachmentDatesByAttachment: {
        [key]: { isOneDay: false, startDate: '05.09.2026', endDate: '05.10.2026', errors: {} },
      },
      byFactPending: true,
    });

    const errors = w.vm.currentAttachmentErrors;
    expect(errors.startDate).toBe(BY_FACT_ONE_DAY_HINT);
    expect(errors.endDate, 'красными становятся обе даты - не годится сам диапазон').toBe(BY_FACT_ONE_DAY_HINT);
    w.unmount();
  });

  it('однодневный период ошибку не поднимает', async () => {
    const w = mountForm();
    const key = w.vm.attachmentKey({ local_id: 'a1', attachment_type: 'cars' });
    await w.setData({
      attachments: [{ local_id: 'a1', attachment_type: 'cars', display_name: 'Авто' }],
      selectedAttachment: { local_id: 'a1', attachment_type: 'cars', display_name: 'Авто' },
      attachmentDatesByAttachment: { [key]: { isOneDay: true, singleDate: '05.09.2026', errors: {} } },
      byFactPending: true,
    });

    expect(w.vm.currentAttachmentErrors.startDate).toBeUndefined();
    w.unmount();
  });
});
