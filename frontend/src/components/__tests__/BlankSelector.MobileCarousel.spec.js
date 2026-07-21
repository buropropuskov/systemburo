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

describe('BlankSelector: карусель типов на телефоне vs колонки на десктопе (#1097 S1)', () => {
  let origMatchMedia;

  beforeEach(() => {
    setActivePinia(createPinia());
    origMatchMedia = window.matchMedia;
  });

  afterEach(() => {
    window.matchMedia = origMatchMedia;
  });

  it('на телефоне типы едут в карусель, и у каждого чипа есть своя кнопка «Добавить»', async () => {
    mockMatchMedia(true);
    const w = await mountSelector();

    expect(w.vm.isNarrow).toBe(true);
    expect(w.find('.category-carousel').exists()).toBe(true);

    const chips = w.findAll('.category-chip');
    expect(chips.length).toBe(w.vm.uniqueCategories.length);
    // Кнопка добавления обязана жить в чипе: без неё на телефоне вложение не создать.
    expect(w.findAll('.category-chip .add-btn').length).toBe(chips.length);
    // Дублей кнопки в списке созданных вложений быть не должно.
    expect(w.findAll('.category .add-btn').length).toBe(0);
    expect(w.findAll('.category-header').length).toBe(0);
  });

  it('на телефоне кнопка чипа реально добавляет вложение выбранного типа', async () => {
    mockMatchMedia(true);
    const w = await mountSelector();

    await w.findAll('.category-chip .add-btn')[0].trigger('click');

    const added = w.emitted('attachment-added');
    expect(added).toBeTruthy();
    expect(added[0][0].title).toBe(w.vm.uniqueCategories[0]);
  });

  it('на десктопе карусели нет, кнопка «Добавить» остаётся в колонке типа', async () => {
    mockMatchMedia(false);
    const w = await mountSelector();

    expect(w.vm.isNarrow).toBe(false);
    expect(w.find('.category-carousel').exists()).toBe(false);
    expect(w.find('.created-caption').exists()).toBe(false);
    expect(w.findAll('.category .add-btn').length).toBe(w.vm.uniqueCategories.length);
    expect(w.findAll('.category-header').length).toBe(w.vm.uniqueCategories.length);
  });
});
