import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({
    ok: true,
    json: vi.fn().mockResolvedValue([
      { id: 1, title: 'Автозаявки', name: 'cars', display_name: 'Автозаявки', attachment_type: 'cars' },
      { id: 2, title: 'Ввоз', name: 'items', display_name: 'Ввоз', attachment_type: 'items' },
    ]),
  }),
}));

import BlankSelector from '../BlankSelector.vue';

// jsdom не реализует matchMedia - без мока initNarrowWatcher выходит по гарду и
// isNarrow навсегда false, то есть мобильная ветка рендера не проверяется.
function mockMatchMedia(matches) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

const ATTACHMENTS = [
  { local_id: 'a1', title: 'Автозаявки', name: 'cars_1', display_name: 'Автозаявка №1', attachment_type: 'cars' },
];

async function mountSelector(attachments = ATTACHMENTS) {
  const w = mount(BlankSelector, {
    props: { attachments },
    global: { stubs: { ConfirmationModal: true } },
  });
  await flushPromises();
  return w;
}

describe('BlankSelector: выбор типа строкой на телефоне, колонки на десктопе (#1097 S1)', () => {
  let origMatchMedia;

  beforeEach(() => {
    setActivePinia(createPinia());
    origMatchMedia = window.matchMedia;
  });

  afterEach(() => {
    window.matchMedia = origMatchMedia;
  });

  it('на телефоне типы едут в строку выбора, добавляет одна кнопка под ней', async () => {
    mockMatchMedia(true);
    const w = await mountSelector();

    expect(w.vm.isNarrow).toBe(true);
    expect(w.find('.category-carousel').exists()).toBe(true);
    expect(w.findAll('.category-chip').length).toBe(w.vm.uniqueCategories.length);
    // Кнопка добавления одна на всю строку: в чипах её быть не должно, иначе
    // она вылезала за их границы и дублировалась по числу типов.
    expect(w.findAll('.category-chip .add-btn').length).toBe(0);
    expect(w.findAll('.category .add-btn').length).toBe(0);
    expect(w.find('.picker-add').exists()).toBe(true);
    expect(w.findAll('.category-header').length).toBe(0);
  });

  it('первый тип выбран сразу, кнопка добавляет вложение выбранного типа', async () => {
    mockMatchMedia(true);
    const w = await mountSelector();

    expect(w.vm.pickedCategory).toBe(w.vm.uniqueCategories[0]);
    await w.find('.picker-add').trigger('click');

    const added = w.emitted('attachment-added');
    expect(added).toBeTruthy();
    expect(added[0][0].title).toBe(w.vm.uniqueCategories[0]);
  });

  it('выбор другого чипа переключает тип, который добавит кнопка', async () => {
    mockMatchMedia(true);
    const w = await mountSelector();
    const second = w.vm.uniqueCategories[1];

    await w.findAll('.category-chip')[1].trigger('click');
    expect(w.vm.pickedCategory).toBe(second);

    await w.find('.picker-add').trigger('click');
    expect(w.emitted('attachment-added')[0][0].title).toBe(second);
  });

  it('на десктопе строки выбора нет, кнопка «Добавить» остаётся в колонке типа', async () => {
    mockMatchMedia(false);
    const w = await mountSelector();

    expect(w.vm.isNarrow).toBe(false);
    expect(w.find('.category-carousel').exists()).toBe(false);
    expect(w.find('.picker-add').exists()).toBe(false);
    expect(w.find('.created-caption').exists()).toBe(false);
    expect(w.findAll('.category .add-btn').length).toBe(w.vm.uniqueCategories.length);
    expect(w.findAll('.category-header').length).toBe(w.vm.uniqueCategories.length);
  });

  it('переключение типа снимает отметки со скрывшихся вложений', async () => {
    mockMatchMedia(true);
    const w = await mountSelector([
      { local_id: 'a1', title: 'Автозаявки', name: 'cars_1', display_name: 'Автозаявка №1', attachment_type: 'cars' },
    ]);

    // отметили вложение первого типа
    w.vm.selectedAttachments = ['a1'];
    // переключились на другой тип - строка a1 скрылась вместе с отметкой
    await w.findAll('.category-chip')[1].trigger('click');
    expect(w.vm.selectedAttachments).toEqual([]);
  });
});
