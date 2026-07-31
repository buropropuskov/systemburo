import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import GlobalSearchPalette from '../GlobalSearchPalette.vue';
import { globalSearch } from '@/api/search';

// Проверяем три вещи, которые ломаются незаметно: разделы находятся без обращения к
// серверу (ради них поиск и открывают), результаты сервера показываются группами, а
// переход закрывает окно ДО навигации -- иначе подтверждение о несохранённой форме
// нарисуется ниже окна по слоям и останется невидимым.

vi.mock('@/api/search', () => ({ globalSearch: vi.fn() }));
vi.mock('@/composables/usePermission', () => ({
  usePermission: () => ({ can: () => true }),
}));

const push = vi.fn();

function mountPalette() {
  return mount(GlobalSearchPalette, {
    props: { show: true },
    global: {
      mocks: { $router: { push } },
      stubs: {
        NavIcon: true,
        SkeletonLine: true,
        // BaseModal телепортирует содержимое в body; для проверок достаточно
        // отрисовать слоты на месте.
        BaseModal: {
          template: '<div class="modal"><slot name="header" /><slot /></div>',
        },
      },
    },
  });
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.useFakeTimers();
  globalSearch.mockResolvedValue({ groups: [], total: 0 });
});

afterEach(() => {
  vi.useRealTimers();
  wrapper?.unmount();
  vi.clearAllMocks();
});

describe('GlobalSearchPalette', () => {
  it('находит раздел меню без обращения к серверу', async () => {
    wrapper = mountPalette();

    await wrapper.setData({ query: 'Автомоб' });

    const titles = wrapper.findAll('.gsp__row-title').map((el) => el.text());
    expect(titles).toContain('Автомобили');
    expect(globalSearch).not.toHaveBeenCalled();
  });

  it('короткий запрос не уходит на сервер', async () => {
    wrapper = mountPalette();

    await wrapper.setData({ query: 'ро' });
    vi.advanceTimersByTime(1000);
    await flushPromises();

    expect(globalSearch).not.toHaveBeenCalled();
  });

  it('запрос уходит один раз после паузы, а не на каждый символ', async () => {
    wrapper = mountPalette();

    await wrapper.setData({ query: 'Рог' });
    await wrapper.setData({ query: 'Рогол' });
    await wrapper.setData({ query: 'Роголев' });
    vi.advanceTimersByTime(400);
    await flushPromises();

    expect(globalSearch).toHaveBeenCalledTimes(1);
    expect(globalSearch).toHaveBeenCalledWith('Роголев', expect.anything());
  });

  it('показывает группы, пришедшие с сервера', async () => {
    globalSearch.mockResolvedValue({
      groups: [{
        type: 'employees',
        title: 'Сотрудники',
        count: 1,
        items: [{ id: 7, type: 'employees', title: 'Роголев Иван', subtitle: 'водитель', target: { entity: 'unique_employee', id: 7 } }],
      }],
      total: 1,
    });
    wrapper = mountPalette();

    await wrapper.setData({ query: 'Роголев' });
    vi.advanceTimersByTime(400);
    await flushPromises();

    expect(wrapper.text()).toContain('Сотрудники');
    expect(wrapper.text()).toContain('Роголев Иван');
  });

  it('сообщает о разделе, который не ответил', async () => {
    globalSearch.mockResolvedValue({ groups: [], total: 0, degraded: ['applications'] });
    wrapper = mountPalette();

    await wrapper.setData({ query: 'Роголев' });
    vi.advanceTimersByTime(400);
    await flushPromises();

    expect(wrapper.text()).toContain('Заявки');
  });

  it('переход закрывает окно раньше навигации', async () => {
    wrapper = mountPalette();
    await wrapper.setData({ query: 'Автомоб' });

    // Проверяем сам инвариант, а не число тиков: к моменту навигации окно уже должно
    // быть закрыто. Иначе подтверждение о несохранённой форме, которое поднимает
    // роутер, нарисуется ниже окна по слоям и останется невидимым, а переход повиснет.
    let closedBeforeNavigation = false;
    push.mockImplementation(() => {
      closedBeforeNavigation = Boolean(wrapper.emitted('close'));
    });

    await wrapper.find('.gsp__row').trigger('click');
    await flushPromises();

    expect(push).toHaveBeenCalledWith({ path: '/carsview' });
    expect(closedBeforeNavigation).toBe(true);
  });
});
