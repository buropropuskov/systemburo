import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import BlacklistTabBase from '../BlacklistTabBase.vue';

const items = [
  { id: 1, is_active: true, reason: 'причина1' },
  { id: 2, is_active: true, reason: 'причина2' },
  { id: 3, is_active: false, reason: 'архивная' },
];

function mountBase(overrides = {}) {
  return mount(BlacklistTabBase, {
    props: {
      apiList: vi.fn().mockResolvedValue(items),
      getPrimaryText: (i) => `Запись ${i.id}`,
      getDetailRows: (i) => [{ label: 'Причина', value: i.reason }],
      searchPlaceholder: 'Поиск...',
      emptyNoun: 'записей',
      ...overrides,
    },
    global: {
      stubs: { BaseDropdown: true, SearchComponent: true, RefreshButton: true, LoaderSpinner: true },
    },
  });
}

describe('BlacklistTabBase', () => {
  it('по умолчанию показывает только активные записи', async () => {
    const wrapper = mountBase();
    await flushPromises();
    expect(wrapper.findAll('.bl-row')).toHaveLength(2);
    expect(wrapper.text()).toContain('Запись 1');
    expect(wrapper.text()).not.toContain('Запись 3');
  });

  it('эмитит count = число активных', async () => {
    const wrapper = mountBase();
    await flushPromises();
    expect(wrapper.emitted('count')).toBeTruthy();
    expect(wrapper.emitted('count').at(-1)).toEqual([2]);
  });

  it('режим Архив показывает неактивные с бейджем (архив)', async () => {
    const wrapper = mountBase();
    await flushPromises();
    wrapper.vm.onArchiveModeChange('archive');
    await flushPromises();
    expect(wrapper.findAll('.bl-row')).toHaveLength(1);
    expect(wrapper.text()).toContain('Запись 3');
    expect(wrapper.find('.bl-inactive-badge').exists()).toBe(true);
  });

  it('поиск фильтрует по причине', async () => {
    const wrapper = mountBase();
    await flushPromises();
    wrapper.vm.searchQuery = 'причина2';
    await flushPromises();
    expect(wrapper.findAll('.bl-row')).toHaveLength(1);
    expect(wrapper.text()).toContain('Запись 2');
  });

  it('клик по строке открывает панель деталей', async () => {
    const wrapper = mountBase();
    await flushPromises();
    expect(wrapper.find('.bl-no-selection').exists()).toBe(true);
    await wrapper.findAll('.bl-row')[0].trigger('click');
    expect(wrapper.find('.bl-details').exists()).toBe(true);
    expect(wrapper.find('.bl-details-title').text()).toBe('Запись 1');
  });

  it('кнопка "Создать запись" эмитит create', async () => {
    const wrapper = mountBase();
    await flushPromises();
    const btn = wrapper.findAll('button').find((b) => b.text() === 'Создать запись');
    await btn.trigger('click');
    expect(wrapper.emitted('create')).toBeTruthy();
  });

  it('кнопка "Снять с ЧС" эмитит archive с активной записью', async () => {
    const wrapper = mountBase();
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    const btn = wrapper.findAll('button').find((b) => b.text() === 'Снять с ЧС');
    await btn.trigger('click');
    expect(wrapper.emitted('archive')[0][0].id).toBe(1);
  });

  it('кнопка "Вернуть в ЧС" эмитит restore для архивной записи', async () => {
    const wrapper = mountBase();
    await flushPromises();
    wrapper.vm.onArchiveModeChange('archive');
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    const btn = wrapper.findAll('button').find((b) => b.text() === 'Вернуть в ЧС');
    await btn.trigger('click');
    expect(wrapper.emitted('restore')[0][0].id).toBe(3);
  });

  it('пустой список показывает empty-state', async () => {
    const wrapper = mountBase({ apiList: vi.fn().mockResolvedValue([]) });
    await flushPromises();
    expect(wrapper.find('.bl-empty').exists()).toBe(true);
  });
});
