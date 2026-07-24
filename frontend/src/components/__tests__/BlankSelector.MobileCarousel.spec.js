import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({
    ok: true,
    json: vi.fn().mockResolvedValue([
      // title - группа (рубрика), display_name - наименование вложения, из которого
      // addAttachment строит «Автозаявка №N»; поля намеренно разные, чтобы тексты
      // «кнопка = вложение, подпись = группа» нельзя было перепутать.
      { id: 1, title: 'Автозаявки', name: 'cars', display_name: 'Автозаявка', attachment_type: 'cars' },
      { id: 2, title: 'Ввоз', name: 'items', display_name: 'Заявка на ввоз', attachment_type: 'items' },
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

  it('на телефоне - одна кнопка «Добавить вложение» с меню, без карусели-переключателя', async () => {
    mockMatchMedia(true);
    const w = await mountSelector();

    expect(w.vm.isNarrow).toBe(true);
    // карусель типов убрана
    expect(w.find('.category-carousel').exists()).toBe(false);
    expect(w.findAll('.category-chip').length).toBe(0);
    // одна кнопка добавления с общим текстом (не «Добавить: тип»)
    const btn = w.find('[data-testid="picker-add"]');
    expect(btn.exists()).toBe(true);
    expect(btn.text()).toContain('Добавить вложение');
    // меню закрыто по умолчанию
    expect(w.find('[data-testid="picker-add-menu"]').exists()).toBe(false);
    // per-column кнопки в самих секциях на мобилке скрыты
    expect(w.findAll('.category .add-btn').length).toBe(0);
  });

  it('клик по кнопке открывает меню со всеми типами (наименования вложений)', async () => {
    mockMatchMedia(true);
    const w = await mountSelector();

    await w.find('[data-testid="picker-add"]').trigger('click');
    expect(w.vm.addMenuOpen).toBe(true);
    expect(w.find('[data-testid="picker-add-menu"]').exists()).toBe(true);

    const items = w.findAll('.picker-add-menu__item');
    expect(items.length).toBe(w.vm.uniqueCategories.length);
    // пункты называют ВЛОЖЕНИЕ (display_name), а не группу капсом
    expect(items[0].text()).toContain('Автозаявка');
    expect(items[1].text()).toContain('Заявка на ввоз');
  });

  it('выбор пункта меню добавляет вложение этого типа и закрывает меню', async () => {
    mockMatchMedia(true);
    const w = await mountSelector();

    await w.find('[data-testid="picker-add"]').trigger('click');
    await w.findAll('.picker-add-menu__item')[1].trigger('click');

    const added = w.emitted('attachment-added');
    expect(added).toBeTruthy();
    expect(added[0][0].title).toBe(w.vm.uniqueCategories[1]);
    // меню закрылось после выбора
    expect(w.vm.addMenuOpen).toBe(false);
    expect(w.find('[data-testid="picker-add-menu"]').exists()).toBe(false);
  });

  it('на телефоне список показывает секции типов с заголовком (общий список, не переключаемый)', async () => {
    mockMatchMedia(true);
    const w = await mountSelector();

    // заголовки типов видны в общем списке (не скрыты как на карусельной версии)
    expect(w.find('.category-header').exists()).toBe(true);
    expect(w.find('.category-title').exists()).toBe(true);
  });

  it('на десктопе меню-кнопки нет, кнопки «Добавить» в колонках типов', async () => {
    mockMatchMedia(false);
    const w = await mountSelector();

    expect(w.vm.isNarrow).toBe(false);
    expect(w.find('[data-testid="picker-add"]').exists()).toBe(false);
    expect(w.find('.category-carousel').exists()).toBe(false);
    expect(w.findAll('.category .add-btn').length).toBe(w.vm.uniqueCategories.length);
    expect(w.findAll('.category-header').length).toBe(w.vm.uniqueCategories.length);
  });

  describe('переименование на телефоне: blur отменяет отложенно', () => {
    it('тап по галочке успевает сохранить, даже если blur пришёл первым', async () => {
      vi.useFakeTimers();
      mockMatchMedia(true);
      const w = await mountSelector();

      w.vm.startRename(ATTACHMENTS[0]);
      w.vm.editingName = 'Моя машина';
      // порядок реального браузера при тапе: сперва blur инпута, затем обработчик кнопки
      w.vm.onRenameBlur(ATTACHMENTS[0]);
      w.vm.commitRename(ATTACHMENTS[0]);

      const ev = w.emitted('attachment-renamed');
      expect(ev).toBeTruthy();
      expect(ev[0][0].display_name).toBe('Моя машина');
      // отложенная отмена снята - по таймеру ничего не происходит
      vi.runAllTimers();
      expect(w.vm.editingKey).toBe(null);
      vi.useRealTimers();
    });

    it('без нажатия кнопок blur отменяет по таймеру, имя не эмитится', async () => {
      vi.useFakeTimers();
      mockMatchMedia(true);
      const w = await mountSelector();

      w.vm.startRename(ATTACHMENTS[0]);
      w.vm.editingName = 'Не сохранять';
      w.vm.onRenameBlur(ATTACHMENTS[0]);
      expect(w.vm.editingKey).not.toBe(null);
      vi.runAllTimers();
      expect(w.vm.editingKey).toBe(null);
      expect(w.emitted('attachment-renamed')).toBeFalsy();
      vi.useRealTimers();
    });

    it('на десктопе blur сохраняет сразу, как раньше', async () => {
      mockMatchMedia(false);
      const w = await mountSelector();

      w.vm.startRename(ATTACHMENTS[0]);
      w.vm.editingName = 'Десктопное имя';
      w.vm.onRenameBlur(ATTACHMENTS[0]);
      expect(w.emitted('attachment-renamed')[0][0].display_name).toBe('Десктопное имя');
    });
  });
});
