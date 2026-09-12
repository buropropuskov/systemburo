import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
const apiRequestRaw = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
  apiRequestRaw: (...args) => apiRequestRaw(...args),
}));

const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

import CarsTableHistoryModal from '../CarsTableHistoryModal.vue';

function pageResponse(items, total) {
  return { ok: true, json: async () => ({ success: true, data: items, meta: { total, page: 1, per_page: 50 } }) };
}

function mark(id) {
  return {
    id,
    car_id: 1,
    user_id: 3,
    user_name: 'Иванов И.И.',
    action_type: 'entry',
    created_at: '2026-09-12T10:00:00Z',
    car_number: 'A001AA777',
    car_brand: 'Volvo',
  };
}

function mountModal() {
  return mount(CarsTableHistoryModal, {
    props: { cars: [], tableId: 7 },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

// Сбой журнала обрабатывается на месте: человек видит тост, а промис не остаётся
// отклонённым. Проброс наружу здесь некому поймать - загрузку начинают mounted и
// обработчики фильтров, - и vitest считает такое отклонение ошибкой прогона: именно так
// этот дефект и вскрылся после мержа (#2469).
describe('CarsTableHistoryModal - сбои загрузки (#2469)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequestRaw.mockReset();
    notify.mockReset();
  });

  it('сбой первой страницы показывает тост и не оставляет отклонённый промис', async () => {
    apiRequestRaw.mockRejectedValue(new Error('сеть отвалилась'));
    apiRequest.mockRejectedValue(new Error('сеть отвалилась'));

    const wrapper = mountModal();
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'историю проходов', type: 'error' }));
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'список пользователей фильтра', type: 'error' }));
    expect(wrapper.findAll('.history-item')).toHaveLength(0);
  });

  // Провалившаяся подгрузка возвращает счётчик страниц назад: иначе следующая попытка
  // запросит третью страницу и в списке появится дыра на месте второй.
  it('после сбоя подгрузки повтор просит ту же страницу', async () => {
    apiRequest.mockResolvedValue({ ok: true, json: async () => ({ users: [] }) });
    apiRequestRaw.mockResolvedValue(pageResponse([mark(1)], 10));

    const wrapper = mountModal();
    await flushPromises();

    apiRequestRaw.mockRejectedValueOnce(new Error('сеть отвалилась'));
    await wrapper.find('.load-more-btn').trigger('click');
    await flushPromises();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'историю проходов', type: 'error' }));

    apiRequestRaw.mockResolvedValue(pageResponse([mark(2)], 10));
    await wrapper.find('.load-more-btn').trigger('click');
    await flushPromises();

    const retried = new URLSearchParams(apiRequestRaw.mock.calls.at(-1)[0].split('?')[1]);
    expect(retried.get('page')).toBe('2');
    expect(wrapper.findAll('.history-item')).toHaveLength(2);
  });

  it('сбой выгрузки не оставляет кнопку в состоянии загрузки', async () => {
    apiRequest.mockResolvedValue({ ok: true, json: async () => ({ users: [] }) });
    apiRequestRaw.mockResolvedValue(pageResponse([mark(1)], 1));

    const wrapper = mountModal();
    await flushPromises();

    apiRequestRaw.mockRejectedValue(new Error('сеть отвалилась'));
    await wrapper.find('.export-btn').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'Ошибка при экспорте в Excel', type: 'error' }));
    expect(wrapper.find('.export-btn').attributes('disabled')).toBeUndefined();
  });
});
