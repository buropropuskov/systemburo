import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) }),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import ApplicationHistory from '../ApplicationHistory.vue';

describe('ApplicationHistory — подача дополнения и номер раунда (#1685)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  const mountHistory = () => mount(ApplicationHistory, {
    props: { applicationId: 12 },
    global: { stubs: { LoaderSpinner: true, teleport: true } },
  });

  it('supplement_created показывает человеческий текст, а не сырой ключ', () => {
    const wrapper = mountHistory();
    const item = { action_type: 'supplement_created' };

    expect(wrapper.vm.getActionText(item)).toBe('Подал(-а) дополнение');
    expect(wrapper.vm.getActionClass('supplement_created')).toBe('dot-create');
  });

  // Метаданные аудита кладут номер в ключ `number` (supplementAuditMetadata на бэке),
  // уведомления - в `supplement_number`. Читаем оба, чтобы подпись не осталась без номера.
  it.each(['number', 'supplement_number'])('номер раунда берётся из metadata.%s', (key) => {
    const wrapper = mountHistory();
    const item = { action_type: 'supplement_created', metadata: { [key]: 2 } };

    expect(wrapper.vm.getActionText(item)).toBe('Подал(-а) дополнение (№2)');
  });

  it('номер дописывается и к остальным событиям раунда', () => {
    const wrapper = mountHistory();

    expect(wrapper.vm.getActionText({ action_type: 'supplement_approve', metadata: { number: 3 } }))
      .toBe('Согласовал(-а) дополнение (№3)');
    expect(wrapper.vm.getActionText({ action_type: 'supplement_confirmation_change', metadata: { number: 3 } }))
      .toBe('Статус согласования дополнения изменился (№3)');
    expect(wrapper.vm.getActionText({ action_type: 'supplement_cancelled', metadata: { number: 1 } }))
      .toBe('Дополнение снято: заявка закрыта (№1)');
  });

  it('без метаданных подпись остаётся прежней', () => {
    const wrapper = mountHistory();

    expect(wrapper.vm.getActionText({ action_type: 'supplement_approve' }))
      .toBe('Согласовал(-а) дополнение');
    expect(wrapper.vm.getActionText({ action_type: 'supplement_created', metadata: {} }))
      .toBe('Подал(-а) дополнение');
  });

  it('номер к событиям самой заявки не приписывается', () => {
    const wrapper = mountHistory();

    expect(wrapper.vm.getActionText({ action_type: 'approve', metadata: { number: 2 } }))
      .toBe('Согласовал(-а) заявку');
  });

  it('мусорный номер в метаданных игнорируется', () => {
    const wrapper = mountHistory();

    expect(wrapper.vm.getActionText({ action_type: 'supplement_created', metadata: { number: 'ой' } }))
      .toBe('Подал(-а) дополнение');
    expect(wrapper.vm.getActionText({ action_type: 'supplement_created', metadata: { number: 0 } }))
      .toBe('Подал(-а) дополнение');
  });
});
