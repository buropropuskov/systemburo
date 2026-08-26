import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) }),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import ApplicationHistory from '../ApplicationHistory.vue';

describe('ApplicationHistory - лейблы решения по дополнению (#1685)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  const mountHistory = () => mount(ApplicationHistory, {
    props: { applicationId: 12 },
    global: { stubs: { LoaderSpinner: true, teleport: true } },
  });

  // Решение по раунду - отдельные действия от принятия самой заявки: "Принял(-а) заявку"
  // в ленте означало бы смену её статуса, а дополнение статус заявки не двигает.
  // Снятие автором отделено от системного "Дополнение снято: заявка закрыта" по той же
  // причине - у первого есть человек-автор, у второго актора нет вовсе.
  it.each([
    ['supplement_accepted', 'Принял(-а) дополнение', 'dot-success'],
    ['supplement_refused', 'Отклонил(-а) дополнение', 'dot-reject'],
    ['supplement_cancelled_by_author', 'Снял(-а) своё дополнение', 'dot-warning'],
  ])('action_type "%s" -> русский лейбл, не сырой ключ', (actionType, text, dotClass) => {
    const wrapper = mountHistory();
    expect(wrapper.vm.getActionText({ action_type: actionType })).toBe(text);
    expect(wrapper.vm.getActionClass(actionType)).toBe(dotClass);
  });
});
