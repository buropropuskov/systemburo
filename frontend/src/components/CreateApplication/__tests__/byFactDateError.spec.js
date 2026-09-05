import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import { BY_FACT_PERIOD_RULE } from '@/utils/byFactVehicle';

/**
 * Правило «срок один день» показывается в общей панели предупреждений, ещё до
 * того как машину «По факту» положат в список (#2320).
 *
 * Отдельного сообщения под полями дат нет намеренно: предупреждения формы живут
 * в одной панели (расписание мест, свободный текст), и второе место для той же
 * мысли только дробило бы внимание.
 *
 * Замок смонтированный, а не текстовый: предыдущая проверка сверяла исходник и
 * пропустила нерабочий код - флаг byFactPending использовался в шаблоне и в
 * computed, но не был объявлен в data. Vue молча отдаёт undefined, предупреждение
 * не появлялось, а тест по исходнику оставался зелёным.
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
  createExtendedTimeoutSignal: vi.fn(() => undefined),
}));

import CreateApplication from '../CreateApplication.vue';

/** Сегодняшняя дата в формате формы: срок «сегодня» всегда в пределах крайней даты. */
function todayRu() {
  const d = new Date();
  const p = (n) => String(n).padStart(2, '0');
  return `${p(d.getDate())}.${p(d.getMonth() + 1)}.${d.getFullYear()}`;
}

function mountForm() {
  setActivePinia(createPinia());
  return shallowMount(CreateApplication, {
    global: { mocks: { $route: { query: {} }, $router: { replace: vi.fn(), push: vi.fn() } } },
  });
}

describe('CreateApplication — предупреждение о сроке машины «По факту»', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('флаг включённого тумблера существует и по умолчанию выключен', () => {
    const w = mountForm();
    expect(w.vm.byFactPending, 'byFactPending обязан быть объявлен в data').toBe(false);
    w.unmount();
  });

  it('включённый тумблер поднимает предупреждение в панели', async () => {
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

    expect(
      w.vm.warningGroups.some((g) => g.free === BY_FACT_PERIOD_RULE),
      'правило должно попасть в панель предупреждений',
    ).toBe(true);

    // Под полями дат сообщения быть не должно: предупреждение живёт в панели,
    // а поля лишь краснеют по флагу без текста.
    expect(w.vm.currentAttachmentErrors.startDate).toBeUndefined();
    expect(w.vm.currentAttachmentErrors.endDate).toBeUndefined();
    expect(w.vm.currentAttachmentErrors.periodInvalid, 'поля дат должны покраснеть').toBe(true);
    w.unmount();
  });

  it('срок в пределах крайней даты предупреждения не поднимает', async () => {
    const w = mountForm();
    const key = w.vm.attachmentKey({ local_id: 'a1', attachment_type: 'cars' });
    await w.setData({
      attachments: [{ local_id: 'a1', attachment_type: 'cars', display_name: 'Авто' }],
      selectedAttachment: { local_id: 'a1', attachment_type: 'cars', display_name: 'Авто' },
      attachmentDatesByAttachment: { [key]: { isOneDay: true, singleDate: todayRu(), errors: {} } },
      byFactPending: true,
    });

    expect(w.vm.warningGroups.some((g) => g.free === BY_FACT_PERIOD_RULE)).toBe(false);
    expect(w.vm.currentAttachmentErrors.periodInvalid, 'в пределах срока поля не краснеют').toBeUndefined();
    w.unmount();
  });

  it('предупреждения по местам панель не теряет', async () => {
    // Правило добавляется к тому, что панель уже показывает, а не вместо него.
    const w = mountForm();
    const key = w.vm.attachmentKey({ local_id: 'a1', attachment_type: 'cars' });
    await w.setData({
      attachments: [{ local_id: 'a1', attachment_type: 'cars', display_name: 'Авто' }],
      selectedAttachment: { local_id: 'a1', attachment_type: 'cars', display_name: 'Авто' },
      attachmentDatesByAttachment: {
        [key]: { isOneDay: false, startDate: '05.09.2026', endDate: '05.10.2026', errors: {} },
      },
      placeNotices: [{ name: 'Дебаркадер №1', free: 'Въезд через ПОСТ №72' }],
      byFactPending: true,
    });

    const имена = w.vm.warningGroups.map((g) => g.name);
    expect(имена).toContain('Дебаркадер №1');
    expect(имена).toContain('Машина «По факту»');
    w.unmount();
  });
});
