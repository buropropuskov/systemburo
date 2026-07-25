import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

import ApplicationAttachmentDetail from '../ApplicationAttachmentDetail.vue';
import { assignElementTables, assignCarUnloadPlaces } from '@/api/applicationAssignments';

vi.mock('@/api/applicationAssignments', () => ({
  assignElementTables: vi.fn().mockResolvedValue({}),
  assignCarUnloadPlaces: vi.fn().mockResolvedValue({}),
}));

function car(over = {}) {
  return {
    id: 1,
    car_number: 'У 952 ЕУ 935',
    car_brand: 'BMW X5',
    unload_places: [{ id: 5, name: 'Склад №1' }],
    target_tables: [{ id: 7, display_name: 'КПП №4' }],
    ...over,
  };
}

function mountList(props = {}) {
  return mount(ApplicationAttachmentDetail, {
    props: {
      attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
      cars: [car()],
      applicationId: 42,
      ...props,
    },
    global: { stubs: { ApplicationAssignModal: true } },
  });
}

describe('ApplicationAttachmentDetail — доназначение мест принимающим (#1393)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it('без права назначать кнопки нет', () => {
    const wrapper = mountList();
    expect(wrapper.find('[data-testid="attachment-assign-open"]').exists()).toBe(false);
  });

  it('принимающему кнопка доступна в колонках мест и постов', () => {
    const wrapper = mountList({ canAssign: true });
    const buttons = wrapper.findAll('[data-testid="attachment-assign-open"]');
    // одна для мест разгрузки, одна для проезда
    expect(buttons).toHaveLength(2);
  });

  it('окно живёт в разметке и в закрытом виде: иначе уход не успевает проиграться', () => {
    const wrapper = mountList({ canAssign: true });
    // v-if на компоненте сносил бы его мгновенно, и анимация закрытия не шла
    const modal = wrapper.findComponent({ name: 'ApplicationAssignModal' });
    expect(modal.exists()).toBe(true);
    expect(modal.props('show')).toBe(false);
  });

  it('клик открывает окно с уже назначенным набором', async () => {
    const wrapper = mountList({ canAssign: true });
    await wrapper.findAll('[data-testid="attachment-assign-open"]')[1].trigger('click');

    expect(wrapper.vm.assign.open).toBe(true);
    expect(wrapper.vm.assign.kind).toBe('tables');
    expect(wrapper.vm.assign.currentIds).toEqual([7]);
    expect(wrapper.vm.assign.elementIds).toEqual([1]);
  });

  it('сохранение шлёт выбранный набор режимом replace и просит обновить данные', async () => {
    const wrapper = mountList({ canAssign: true });
    await wrapper.findAll('[data-testid="attachment-assign-open"]')[1].trigger('click');
    await wrapper.vm.applyAssign([7, 9]);

    expect(assignElementTables).toHaveBeenCalledWith(42, {
      elementType: 'cars',
      elementIds: [1],
      tableIds: [7, 9],
      mode: 'replace',
    });
    expect(wrapper.emitted('assignments-changed')).toHaveLength(1);
    expect(wrapper.vm.assign.open).toBe(false);
  });

  it('пустой набор — это снятие всех привязок, запрос всё равно уходит', async () => {
    const wrapper = mountList({ canAssign: true });
    await wrapper.findAll('[data-testid="attachment-assign-open"]')[1].trigger('click');
    await wrapper.vm.applyAssign([]);

    expect(assignElementTables).toHaveBeenCalledWith(expect.anything(), expect.objectContaining({ tableIds: [] }));
  });

  it('места разгрузки уходят своим запросом', async () => {
    const wrapper = mountList({ canAssign: true });
    await wrapper.findAll('[data-testid="attachment-assign-open"]')[0].trigger('click');
    expect(wrapper.vm.assign.kind).toBe('places');

    await wrapper.vm.applyAssign([5, 6]);
    expect(assignCarUnloadPlaces).toHaveBeenCalledWith(42, {
      carIds: [1],
      placeIds: [5, 6],
      mode: 'replace',
    });
  });

  it('ошибка бэка показывается и окно остаётся открытым', async () => {
    assignElementTables.mockRejectedValueOnce(new Error('Заявка в статусе «Завершено»: менять места нельзя'));
    const wrapper = mountList({ canAssign: true });
    await wrapper.findAll('[data-testid="attachment-assign-open"]')[1].trigger('click');
    await wrapper.vm.applyAssign([9]);

    expect(wrapper.emitted('assignments-changed')).toBeUndefined();
    expect(wrapper.vm.assign.open).toBe(true);
  });

  it('«Назначить всем» есть у обеих колонок машин', () => {
    const wrapper = mountList({ canAssign: true });
    expect(wrapper.find('[data-testid="attachment-assign-all-tables"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="attachment-assign-all-places"]').exists()).toBe(true);
  });

  it('без права назначать массовых кнопок нет', () => {
    const wrapper = mountList();
    expect(wrapper.find('[data-testid="attachment-assign-all-tables"]').exists()).toBe(false);
  });

  it('массовое назначение берёт все строки, а отмеченным показывает общее для всех', async () => {
    const wrapper = mountList({
      canAssign: true,
      cars: [
        car({ id: 1, target_tables: [{ id: 7, display_name: 'КПП №4' }, { id: 9, display_name: 'ПОСТ №72' }] }),
        car({ id: 2, target_tables: [{ id: 7, display_name: 'КПП №4' }] }),
      ],
    });
    await wrapper.find('[data-testid="attachment-assign-all-tables"]').trigger('click');

    expect(wrapper.vm.assign.elementIds).toEqual([1, 2]);
    // у первой машины два поста, у второй один - общий только КПП №4
    expect(wrapper.vm.assign.currentIds).toEqual([7]);
  });

  it('массовое назначение применяется к найденным строкам, а не ко всем', async () => {
    const wrapper = mountList({
      canAssign: true,
      cars: [
        car({ id: 1, car_number: 'У 952 ЕУ 935' }),
        car({ id: 2, car_number: 'М 234 ОО 123' }),
      ],
    });
    await wrapper.find('[data-testid="attachment-elements-search"] input').setValue('952');
    await wrapper.find('[data-testid="attachment-assign-all-tables"]').trigger('click');

    expect(wrapper.vm.assign.elementIds).toEqual([1]);
  });

  it('сохранение массового назначения шлёт все выбранные строки', async () => {
    const wrapper = mountList({
      canAssign: true,
      cars: [car({ id: 1 }), car({ id: 2 })],
    });
    await wrapper.find('[data-testid="attachment-assign-all-places"]').trigger('click');
    await wrapper.vm.applyAssign([5]);

    expect(assignCarUnloadPlaces).toHaveBeenCalledWith(42, {
      carIds: [1, 2],
      placeIds: [5],
      mode: 'replace',
    });
  });

  it('в пустой колонке кнопка подписана словом, а не значком', () => {
    const wrapper = mountList({
      canAssign: true,
      cars: [car({ id: 1, unload_places: [], target_tables: [] })],
    });
    const buttons = wrapper.findAll('[data-testid="attachment-assign-open"]');
    expect(buttons[0].text()).toBe('Добавить');
    expect(buttons[0].attributes('data-hint')).toBe('Назначить места разгрузки');
  });

  it('когда что-то назначено, кнопка компактная и подсказка про изменение', () => {
    const wrapper = mountList({ canAssign: true });
    const buttons = wrapper.findAll('[data-testid="attachment-assign-open"]');
    expect(buttons[0].text()).toBe('+');
    expect(buttons[0].attributes('data-hint')).toBe('Изменить места разгрузки');
  });

  it('у сотрудников кнопка только в колонке мест прохода', () => {
    const wrapper = mount(ApplicationAttachmentDetail, {
      props: {
        attachment: { id: 2, attachment_type: 'people', attachment_display_name: 'Люди' },
        employees: [{ id: 3, last_name: 'Иванов', first_name: 'Иван', target_tables: [] }],
        applicationId: 42,
        canAssign: true,
      },
      global: { stubs: { ApplicationAssignModal: true } },
    });
    expect(wrapper.findAll('[data-testid="attachment-assign-open"]')).toHaveLength(1);
  });

  it('у ТМЦ назначать нечего', () => {
    const wrapper = mount(ApplicationAttachmentDetail, {
      props: {
        attachment: { id: 3, attachment_type: 'items', attachment_display_name: 'Имущество' },
        items: [{ id: 4, name: 'Ноутбук', count: 1 }],
        applicationId: 42,
        canAssign: true,
      },
      global: { stubs: { ApplicationAssignModal: true } },
    });
    expect(wrapper.find('[data-testid="attachment-assign-open"]').exists()).toBe(false);
  });
});
