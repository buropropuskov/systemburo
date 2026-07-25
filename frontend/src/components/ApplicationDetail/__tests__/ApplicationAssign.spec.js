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
