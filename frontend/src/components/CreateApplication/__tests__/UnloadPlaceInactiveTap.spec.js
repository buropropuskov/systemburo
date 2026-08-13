import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import ItemsForm from '../ItemsForm.vue';
import VehicleForm from '../VehicleForm.vue';

// Причина недоступности места разгрузки жила только в hover-подсказке, то есть на
// телефоне была недостижима. Теперь тап по закрытой плитке показывает её сам и гасит
// через 2.5 с - как это давно делает грид постов (TargetTablesGrid).

const PLACES = [
  { id: 1, name: 'Ворота 1', status: 'active' },
  { id: 2, name: 'Док 3', status: 'inactive', status_comment: 'ремонт до пятницы' },
  { id: 3, name: 'Док 4', status: 'inactive' },
];

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn((url) => {
    if (url === '/unload-places') {
      return Promise.resolve({ ok: true, json: async () => PLACES });
    }
    return Promise.resolve({ ok: true, json: async () => [] });
  }),
}));
vi.mock('@/api/blacklist', () => ({ checkVehicleBlacklist: vi.fn().mockResolvedValue(null) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn(() => ({ token: 'test-token' })) }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify: vi.fn(), enqueue: vi.fn() })) }));
vi.mock('@/api/marks', () => ({ listMarks: vi.fn().mockResolvedValue([]) }));

async function mountItems() {
  const w = mount(ItemsForm, {
    props: { showUnloadPlaces: true, allUnloadingPlaces: PLACES, selectedUnloadPlaces: [] },
  });
  await flushPromises();
  return w;
}

async function mountVehicle() {
  const w = mount(VehicleForm);
  await flushPromises();
  w.vm.allUnloadingPlaces = PLACES;
  await w.vm.$nextTick();
  return w;
}

describe.each([
  ['ItemsForm', mountItems],
  ['VehicleForm', mountVehicle],
])('%s - причина недоступности места разгрузки по тапу', (_name, mountForm) => {
  beforeEach(() => { vi.clearAllMocks(); });

  it('тап по закрытому месту показывает причину и не выбирает его', async () => {
    const w = await mountForm();

    await w.findAll('.unloading__item')[1].trigger('click');
    await w.vm.$nextTick();

    expect(w.find('.inactive-tooltip-content').text()).toBe('Недоступно: ремонт до пятницы');
    expect(w.emitted('update:unload-places')).toBeUndefined();
  });

  it('без указанной причины подсказка говорит просто «Недоступно»', async () => {
    const w = await mountForm();

    await w.findAll('.unloading__item')[2].trigger('click');
    await w.vm.$nextTick();

    expect(w.find('.inactive-tooltip-content').text()).toBe('Недоступно');
  });

  it('тап по доступному месту выбирает его и подсказку не показывает', async () => {
    const w = await mountForm();

    await w.findAll('.unloading__item')[0].trigger('click');
    await w.vm.$nextTick();

    expect(w.emitted('update:unload-places')[0][0]).toEqual([1]);
    expect(w.find('.inactive-tooltip').exists()).toBe(false);
  });

  it('тап по другому закрытому месту продлевает показ, а не гасит его старым таймером', async () => {
    const w = await mountForm();
    vi.useFakeTimers();

    try {
      await w.findAll('.unloading__item')[1].trigger('click');
      await w.vm.$nextTick();

      vi.advanceTimersByTime(2000);
      await w.findAll('.unloading__item')[2].trigger('click');
      await w.vm.$nextTick();

      // Старый таймер отсчитал бы своё через 500 мс и погасил чужую подсказку.
      vi.advanceTimersByTime(1000);
      await w.vm.$nextTick();
      expect(w.find('.inactive-tooltip-content').text()).toBe('Недоступно');

      vi.advanceTimersByTime(1500);
      await w.vm.$nextTick();
      expect(w.find('.inactive-tooltip').exists()).toBe(false);
    } finally {
      vi.useRealTimers();
    }
  });

  it('подсказка гаснет сама через 2.5 с', async () => {
    const w = await mountForm();
    vi.useFakeTimers();

    try {
      await w.findAll('.unloading__item')[1].trigger('click');
      await w.vm.$nextTick();
      expect(w.find('.inactive-tooltip').exists()).toBe(true);

      vi.advanceTimersByTime(2500);
      await w.vm.$nextTick();

      expect(w.find('.inactive-tooltip').exists()).toBe(false);
    } finally {
      vi.useRealTimers();
    }
  });
});
