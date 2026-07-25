import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

import ApplicationAssignModal from '../ApplicationAssignModal.vue';
import { apiRequest } from '@/api/client';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

function okJson(data) {
  return { ok: true, json: vi.fn().mockResolvedValue(data) };
}

const TABLES = [
  { table: { id: 7, name: 'kpp4', display_name: 'КПП №4', table_type: 'cars', status: 'active' } },
  { table: { id: 9, name: 'post72', display_name: 'ПОСТ №72', table_type: 'cars', status: 'active' } },
  { table: { id: 11, name: 'people1', display_name: 'Проходная', table_type: 'people', status: 'active' } },
];

const PLACES = [
  { id: 5, name: 'Склад №1', status: 'active', is_active: true },
  { id: 6, name: 'Рампа А', status: 'active', is_active: true },
  { id: 8, name: 'Ремонтируется', status: 'maintenance', is_active: true },
];

function mountModal(props = {}) {
  return mount(ApplicationAssignModal, {
    props: { show: true, kind: 'tables', elementType: 'cars', ...props },
    global: { stubs: { BaseModal: { template: '<div><slot /><slot name="actions" /></div>' } } },
  });
}

describe('ApplicationAssignModal (#1393)', () => {
  beforeEach(() => vi.clearAllMocks());

  it('скругление как у остальных окон проекта, а не дефолтные 15px', () => {
    apiRequest.mockResolvedValue(okJson([]));
    const wrapper = mount(ApplicationAssignModal, {
      props: { show: true, kind: 'tables', elementType: 'cars' },
      global: { stubs: { BaseModal: { props: ['radius'], template: '<div :data-radius="radius"><slot /></div>' } } },
    });
    expect(wrapper.find('[data-radius]').attributes('data-radius')).toBe('30px');
  });

  it('окно поднято над деталью заявки: с дефолтным слоем оно открывалось за ней', () => {
    apiRequest.mockResolvedValue(okJson([]));
    const wrapper = mount(ApplicationAssignModal, {
      props: { show: true, kind: 'tables', elementType: 'cars' },
      global: { stubs: { BaseModal: { props: ['zIndex'], template: '<div :data-z="zIndex"><slot /></div>' } } },
    });
    // деталь заявки - 10002, карточки из неё - 10003 и 10005
    expect(Number(wrapper.find('[data-z]').attributes('data-z'))).toBeGreaterThan(10005);
  });

  it('показывает посты только своего типа элемента', async () => {
    apiRequest.mockResolvedValue(okJson(TABLES));
    const wrapper = mountModal();
    await flushPromises();

    expect(wrapper.vm.options.map(o => o.table.id)).toEqual([7, 9]);
  });

  it('текущий набор приходит уже отмеченным', async () => {
    apiRequest.mockResolvedValue(okJson(TABLES));
    const wrapper = mountModal({ currentIds: [9] });
    await flushPromises();

    expect(wrapper.vm.selected).toEqual([9]);
  });

  it('сохранение отдаёт родителю итоговый набор', async () => {
    apiRequest.mockResolvedValue(okJson(TABLES));
    const wrapper = mountModal({ currentIds: [7] });
    await flushPromises();

    wrapper.vm.selected = [7, 9];
    await wrapper.find('[data-testid="application-assign-apply"]').trigger('click');
    expect(wrapper.emitted('apply')[0][0]).toEqual([7, 9]);
  });

  it('снятие всех отметок предупреждает о последствии', async () => {
    apiRequest.mockResolvedValue(okJson(TABLES));
    const wrapper = mountModal({ currentIds: [7] });
    await flushPromises();

    expect(wrapper.find('[data-testid="application-assign-warning"]').exists()).toBe(false);
    wrapper.vm.selected = [];
    await wrapper.vm.$nextTick();
    expect(wrapper.find('[data-testid="application-assign-warning"]').text()).toContain('не попадёт ни в одну таблицу');
  });

  it('места разгрузки: клик отмечает, повторный снимает', async () => {
    apiRequest.mockResolvedValue(okJson(PLACES));
    const wrapper = mountModal({ kind: 'places' });
    await flushPromises();

    const items = wrapper.findAll('[data-testid="application-assign-place"]');
    expect(items).toHaveLength(3);

    await items[0].trigger('click');
    expect(wrapper.vm.selected).toEqual([5]);
    await items[0].trigger('click');
    expect(wrapper.vm.selected).toEqual([]);
  });

  it('место на обслуживании выбрать нельзя', async () => {
    apiRequest.mockResolvedValue(okJson(PLACES));
    const wrapper = mountModal({ kind: 'places' });
    await flushPromises();

    await wrapper.findAll('[data-testid="application-assign-place"]')[2].trigger('click');
    expect(wrapper.vm.selected).toEqual([]);
  });

  it('уже назначенный, но отключённый пост остаётся в списке', async () => {
    apiRequest.mockResolvedValue(okJson([
      ...TABLES,
      { table: { id: 13, name: 'old', display_name: 'Старый пост', table_type: 'cars', status: 'inactive', is_active: false } },
    ]));
    // без назначения отключённый не показывается
    let wrapper = mountModal();
    await flushPromises();
    expect(wrapper.vm.options.map(o => o.table.id)).not.toContain(13);

    // а если он уже привязан - показываем, иначе сохранение молча снимет его
    wrapper = mountModal({ currentIds: [13] });
    await flushPromises();
    expect(wrapper.vm.options.map(o => o.table.id)).toContain(13);
  });

  it('при назначении нескольким сообщает, скольких затронет', async () => {
    apiRequest.mockResolvedValue(okJson(TABLES));
    const wrapper = mountModal({ targetCount: 8 });
    await flushPromises();

    expect(wrapper.text()).toContain('применится к 8 машинам');
  });

  it('пока справочник грузится, показываются заглушки, а не голый текст', async () => {
    let release;
    apiRequest.mockReturnValue(new Promise((resolve) => { release = () => resolve(okJson(TABLES)); }));
    const wrapper = mountModal();
    await wrapper.vm.$nextTick();

    expect(wrapper.find('[data-testid="application-assign-loading"]').exists()).toBe(true);
    expect(wrapper.findAllComponents({ name: 'SkeletonBlock' }).length).toBeGreaterThan(0);

    release();
    await flushPromises();
    expect(wrapper.find('[data-testid="application-assign-loading"]').exists()).toBe(false);
  });

  it('заглушек столько же рядов, сколько займут данные: у мест их больше, чем у постов', () => {
    const tables = mountModal({ kind: 'tables' });
    const places = mountModal({ kind: 'places' });
    expect(places.vm.skeletonCount).toBeGreaterThan(tables.vm.skeletonCount);
    // резерв высоты тоже разный - иначе окно прыгает при появлении данных
    expect(places.vm.contentMinHeight).not.toBe(tables.vm.contentMinHeight);
  });

  it('сорванный запрос не роняет окно необработанным промисом', async () => {
    apiRequest.mockRejectedValue(new Error('network'));
    const wrapper = mountModal();
    await flushPromises();

    expect(wrapper.vm.loading).toBe(false);
    expect(wrapper.find('[data-testid="application-assign-empty"]').exists()).toBe(true);
  });

  it('пустой справочник объясняет, что выбирать нечего', async () => {
    apiRequest.mockResolvedValue(okJson([]));
    const wrapper = mountModal();
    await flushPromises();

    expect(wrapper.find('[data-testid="application-assign-empty"]').text()).toContain('Нет доступных постов');
  });
});
