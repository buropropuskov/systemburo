import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const getWorkModes = vi.fn();
vi.mock('@/api/work-modes', () => ({
  getWorkModes: (...a) => getWorkModes(...a),
}));

import WorkModesModal from '../WorkModesModal.vue';

// day_of_week 0 = Понедельник — зашиваем расписание в Пн, чтобы ассертить строку
// недели детерминированно, независимо от того, какой сегодня день.
const MODES = {
  bureau: {
    id: 0,
    kind: 'bureau',
    name: 'Бюро пропусков',
    status: 'active',
    current_status: 'open',
    time_slots: [
      { day_of_week: 0, open_time: '09:00:00', close_time: '18:00:00', is_next_day: false, is_active: true },
    ],
  },
  unload_places: [
    {
      id: 1,
      kind: 'unload_place',
      name: 'Склад №1',
      status: 'active',
      current_status: 'closed',
      time_slots: [
        { day_of_week: 0, open_time: '08:00', close_time: '12:00', is_next_day: false, is_active: true },
        { day_of_week: 0, open_time: '14:00', close_time: '20:00', is_next_day: false, is_active: true },
      ],
    },
    {
      id: 2,
      kind: 'unload_place',
      name: 'Терминал',
      status: 'maintenance',
      current_status: 'closed',
      time_slots: [],
    },
  ],
  checkpoints: [
    {
      id: 5,
      kind: 'checkpoint',
      name: 'КПП-1',
      status: 'active',
      current_status: 'open',
      time_slots: [
        { day_of_week: 0, open_time: '00:00', close_time: '23:59', is_next_day: false, is_active: true },
      ],
    },
    {
      id: 6,
      kind: 'checkpoint',
      name: 'Пост Б',
      status: 'inactive',
      current_status: 'closed',
      time_slots: [],
    },
  ],
};

async function openModal(resolved = MODES) {
  getWorkModes.mockResolvedValue(resolved);
  const wrapper = mount(WorkModesModal, {
    props: { show: false },
    global: { stubs: { teleport: true, LoaderSpinner: true } },
    attachTo: document.body,
  });
  await wrapper.setProps({ show: true });
  await flushPromises();
  return wrapper;
}

// Категории теперь в BaseDropdown (#1097 R3-8): открыть меню и кликнуть опцию.
async function selectCategory(wrapper, key) {
  const idx = { bureau: 0, unload: 1, pass: 2 }[key];
  await wrapper.find('.base-dropdown__button').trigger('click');
  const items = wrapper.findAll('.base-dropdown__item');
  await items[idx].trigger('click');
  await wrapper.vm.$nextTick();
}

describe('WorkModesModal', () => {
  beforeEach(() => {
    getWorkModes.mockReset();
  });

  it('грузит режимы при открытии и рисует 3 категории со счётчиками', async () => {
    const wrapper = await openModal();
    expect(getWorkModes).toHaveBeenCalledTimes(1);
    await wrapper.find('.base-dropdown__button').trigger('click');
    const items = wrapper.findAll('.base-dropdown__item');
    expect(items).toHaveLength(3);
    expect(items[0].text()).toContain('Бюро');
    expect(items[1].text()).toContain('Места разгрузки');
    expect(items[1].text()).toContain('2');
    expect(items[2].text()).toContain('Места прохода');
    expect(items[2].text()).toContain('2');
    // У Бюро счётчик не рисуется
    expect(items[0].find('.cat-opt__cnt').exists()).toBe(false);
  });

  it('по умолчанию активна вкладка Бюро, карточка развёрнута, статус «Открыто сейчас»', async () => {
    const wrapper = await openModal();
    const card = wrapper.find('[data-testid="work-modes-card"]');
    expect(card.find('.obj__name').text()).toBe('Бюро пропусков');
    expect(card.classes()).toContain('obj--open');
    const status = card.find('[data-testid="work-modes-status"]');
    expect(status.text()).toBe('Открыто сейчас');
    expect(status.classes()).toContain('status--open');
  });

  it('недельная сетка: 7 дней, Пн объединяет окна через запятую, ровно один «сегодня»', async () => {
    const wrapper = await openModal();
    await selectCategory(wrapper, 'unload');
    const sklad = wrapper.findAll('[data-testid="work-modes-card"]')[0];
    const rows = sklad.findAll('.week__row');
    expect(rows).toHaveLength(7);
    // Понедельник (индекс 0) — два активных окна, объединённых ", "
    expect(rows[0].find('.week__time').text()).toBe('08:00 – 12:00, 14:00 – 20:00');
    // Дни без активных слотов — «выходной»
    expect(rows[1].find('.week__time').text()).toBe('выходной');
    expect(rows[1].find('.week__time').classes()).toContain('is-off');
    expect(sklad.findAll('.week__row--today')).toHaveLength(1);
  });

  it('статусы: maintenance -> «На обслуживании», inactive -> «Неактивно», closed -> «Закрыто сейчас»', async () => {
    const wrapper = await openModal();

    await selectCategory(wrapper, 'unload');
    const unload = wrapper.findAll('[data-testid="work-modes-status"]');
    expect(unload[0].text()).toBe('Закрыто сейчас'); // active + closed
    expect(unload[0].classes()).toContain('status--closed');
    expect(unload[1].text()).toBe('На обслуживании'); // maintenance
    expect(unload[1].classes()).toContain('status--inactive');

    await selectCategory(wrapper, 'pass');
    const pass = wrapper.findAll('[data-testid="work-modes-status"]');
    expect(pass[1].text()).toBe('Неактивно'); // inactive
    expect(pass[1].classes()).toContain('status--inactive');
  });

  it('круглосуточный слот (00:00-23:59) рисуется как «круглосуточно» зелёным', async () => {
    const wrapper = await openModal();
    await selectCategory(wrapper, 'pass');
    const kpp = wrapper.findAll('[data-testid="work-modes-card"]')[0]; // КПП-1
    await kpp.find('.obj__head').trigger('click'); // в категории 2 поста -> разворачиваем вручную
    expect(kpp.classes()).toContain('obj--open');
    const mondayTime = kpp.findAll('.week__row')[0].find('.week__time');
    expect(mondayTime.text()).toBe('круглосуточно');
    expect(mondayTime.classes()).toContain('is-24');
  });

  it('клик по шапке карточки сворачивает/разворачивает неделю', async () => {
    const wrapper = await openModal();
    const card = wrapper.find('[data-testid="work-modes-card"]'); // Бюро, развёрнуто
    expect(card.classes()).toContain('obj--open');
    await card.find('.obj__head').trigger('click');
    expect(card.classes()).not.toContain('obj--open');
    await card.find('.obj__head').trigger('click');
    expect(card.classes()).toContain('obj--open');
  });

  it('состояние загрузки и ошибки', async () => {
    // loading: запрос ещё не зарезолвлен
    let resolveFn;
    getWorkModes.mockReturnValue(new Promise((res) => { resolveFn = res; }));
    const wrapper = mount(WorkModesModal, {
      props: { show: false },
      global: { stubs: { teleport: true, LoaderSpinner: true } },
      attachTo: document.body,
    });
    await wrapper.setProps({ show: true });
    await wrapper.vm.$nextTick();
    expect(wrapper.find('.modes__state').exists()).toBe(true);
    expect(wrapper.find('.cats__select').exists()).toBe(false);
    resolveFn(MODES);
    await flushPromises();
    expect(wrapper.find('.cats__select').exists()).toBe(true);

    // error: запрос упал
    getWorkModes.mockRejectedValueOnce(new Error('boom'));
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    await flushPromises();
    expect(wrapper.find('.modes__state').text()).toContain('Не удалось загрузить');
  });

  it('эмитит close по «Готово», крестику и Escape', async () => {
    const wrapper = await openModal();
    await wrapper.find('.modes__done').trigger('click');
    await wrapper.find('.modes__close').trigger('click');
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await wrapper.vm.$nextTick();
    expect(wrapper.emitted('close')).toHaveLength(3);
  });
});
