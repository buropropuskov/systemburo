import { describe, it, expect, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';

import ApplicationTags from '@/components/ApplicationTags.vue';

function application(over = {}) {
  return {
    id: 7,
    status: 'В обработке',
    confirmation: 'Согласование',
    sending_datetime: new Date(Date.now() - 6 * 86400000).toISOString(),
    ...over,
  };
}

const ALL_FLAGS = {
  blacklist_flags_count: 3,
  has_roof_access: true,
  has_free_parking: true,
  sender_is_important: true,
  has_unseen_questions: true,
  has_open_supplement: true,
  has_files: true,
};

let wrapper;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

function mountTags(over = {}, availableWidth = 0) {
  return mount(ApplicationTags, {
    props: { application: application(over), availableWidth },
    attachTo: document.body,
  });
}

describe('ApplicationTags', () => {
  it('без признаков не рисует ничего', () => {
    wrapper = mount(ApplicationTags, {
      props: { application: { id: 1, status: 'Завершено', confirmation: 'Согласовано' } },
    });

    expect(wrapper.find('.application-tags').exists()).toBe(false);
  });

  it('держит testid, по которым его находят онбординг и тесты списка', () => {
    wrapper = mountTags({
      blacklist_flags_count: 2,
      has_files: true,
      has_unseen_questions: true,
      has_open_supplement: true,
    });

    expect(wrapper.find('[data-testid="ob-center-blacklist-tag"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="center-files-badge-7"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="center-questions-badge-7"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="center-supplement-badge-7"]').exists()).toBe(true);
  });

  it('в свёрнутом виде ЧС показывает число, а полный текст уходит в подсказку', () => {
    wrapper = mountTags({ blacklist_flags_count: 2 }, 90);
    const chs = wrapper.find('[data-testid="ob-center-blacklist-tag"]');

    expect(chs.classes()).toContain('rt-tag--mode-count');
    expect(chs.find('.rt-tag__text').text()).toBe('2');
    expect(chs.attributes('data-hint')).toContain('чёрный список');
  });

  it('на просторной колонке ЧС остаётся полной подписью', () => {
    wrapper = mountTags({ blacklist_flags_count: 2 }, 168);
    const chs = wrapper.find('[data-testid="ob-center-blacklist-tag"]');

    expect(chs.classes()).toContain('rt-tag--mode-text');
    expect(chs.find('.rt-tag__text').text()).toBe('2 похожи на ЧС');
  });

  it('без ограничения ширины (мобильная карточка) все теги идут подписями', () => {
    wrapper = mountTags({ blacklist_flags_count: 2, has_roof_access: true, has_free_parking: true }, 0);

    expect(wrapper.find('[data-testid="center-tags-more"]').exists()).toBe(false);
    expect(wrapper.findAll('.rt-tag--mode-text')).toHaveLength(4);
  });
});

describe('ApplicationTags — исключение тегов разделом', () => {
  it('исключённый тег не показывается и не занимает места', () => {
    // Личный кабинет прячет «Важный»: отправитель там - сам читающий, и тег висел
    // бы на каждой его строке, ничего не сообщая.
    wrapper = mount(ApplicationTags, {
      props: { application: application(ALL_FLAGS), availableWidth: 0, exclude: ['important'] },
      attachTo: document.body,
    });

    const text = wrapper.text();
    expect(text).not.toContain('Важный');
    expect(text, 'исключение одного тега не должно убирать остальные').toContain('Крыша');
  });

  it('без исключений состав прежний', () => {
    wrapper = mountTags(ALL_FLAGS);
    expect(wrapper.text()).toContain('Важный');
  });
});

describe('ApplicationTags — список скрытых тегов', () => {
  it('счётчик закрыт по умолчанию и раскрывается по клику', async () => {
    wrapper = mountTags(ALL_FLAGS, 90);
    const more = wrapper.find('[data-testid="center-tags-more"]');
    expect(more.exists()).toBe(true);
    expect(document.querySelector('[data-testid="center-tags-popover"]')).toBeNull();

    await more.trigger('click');

    const popover = document.querySelector('[data-testid="center-tags-popover"]');
    expect(popover).not.toBeNull();
    expect(more.attributes('aria-expanded')).toBe('true');
  });

  it('в списке лежат ровно недостающие теги и с полными подписями', async () => {
    wrapper = mountTags(ALL_FLAGS, 90);
    const hidden = wrapper.vm.layout.hidden.map((t) => t.text);
    await wrapper.find('[data-testid="center-tags-more"]').trigger('click');

    const popover = document.querySelector('[data-testid="center-tags-popover"]');
    const shown = Array.from(popover.querySelectorAll('.rt-tag')).map((el) => el.textContent.trim());

    expect(hidden.length).toBeGreaterThan(0);
    expect(shown).toEqual(hidden);
    // Полный вид: подпись не схлопнута в иконку, как в тесной строке.
    expect(popover.querySelectorAll('.rt-tag--mode-text').length).toBe(hidden.length);
  });

  it('повторный клик и Escape закрывают список', async () => {
    wrapper = mountTags(ALL_FLAGS, 90);
    const more = wrapper.find('[data-testid="center-tags-more"]');

    await more.trigger('click');
    await more.trigger('click');
    expect(document.querySelector('[data-testid="center-tags-popover"]')).toBeNull();

    await more.trigger('click');
    expect(document.querySelector('[data-testid="center-tags-popover"]')).not.toBeNull();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await wrapper.vm.$nextTick();
    expect(document.querySelector('[data-testid="center-tags-popover"]')).toBeNull();
  });

  it('клик мимо списка закрывает его', async () => {
    wrapper = mountTags(ALL_FLAGS, 90);
    await wrapper.find('[data-testid="center-tags-more"]').trigger('click');

    document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    await wrapper.vm.$nextTick();

    expect(document.querySelector('[data-testid="center-tags-popover"]')).toBeNull();
  });

  it('клик по счётчику не всплывает до строки: заявка не открывается', async () => {
    let rowClicks = 0;
    const host = mount({
      components: { ApplicationTags },
      data: () => ({ app: application(ALL_FLAGS) }),
      template: '<div @click="onRow"><ApplicationTags :application="app" :available-width="90" /></div>',
      methods: { onRow() { rowClicks += 1; } },
    }, { attachTo: document.body });

    await host.find('[data-testid="center-tags-more"]').trigger('click');

    expect(document.querySelector('[data-testid="center-tags-popover"]')).not.toBeNull();
    expect(rowClicks).toBe(0);
    host.unmount();
  });

  it('у нижней кромки экрана список разворачивается вверх', async () => {
    wrapper = mountTags(ALL_FLAGS, 90);
    await wrapper.find('[data-testid="center-tags-more"]').trigger('click');

    // Чип у самого низа окна: снизу места под список нет, сверху - вдоволь.
    const chip = wrapper.vm.$refs.moreChip.$el;
    chip.getBoundingClientRect = () => ({ top: 700, bottom: 723, left: 1100, right: 1140, width: 40, height: 23 });
    window.innerHeight = 760;
    window.innerWidth = 1440;
    wrapper.vm.positionPopover();

    expect(wrapper.vm.popoverStyle.top).toBe('auto');
    expect(wrapper.vm.popoverStyle.bottom).toBe(`${760 - 700 + 6}px`);
  });

  it('список равняется по правому краю счётчика и не вылезает за окно', async () => {
    wrapper = mountTags(ALL_FLAGS, 90);
    await wrapper.find('[data-testid="center-tags-more"]').trigger('click');

    const chip = wrapper.vm.$refs.moreChip.$el;
    chip.getBoundingClientRect = () => ({ top: 100, bottom: 123, left: 1400, right: 1438, width: 38, height: 23 });
    window.innerHeight = 900;
    window.innerWidth = 1440;
    wrapper.vm.positionPopover();

    const left = parseInt(wrapper.vm.popoverStyle.left, 10);
    const width = parseInt(wrapper.vm.popoverStyle.width, 10);
    expect(width).toBe(wrapper.vm.popoverWidth);
    expect(left + width).toBeLessThanOrEqual(1440);
    expect(left).toBeGreaterThanOrEqual(0);
  });

  it('панель складывает теги в строку: ширина по содержимому, не по числу тегов', async () => {
    wrapper = mountTags(ALL_FLAGS, 90);
    await wrapper.find('[data-testid="center-tags-more"]').trigger('click');

    const hidden = wrapper.vm.layout.hidden;
    const widest = Math.max(...hidden.map((t) => t.text.length));
    // Ширина панели заведомо больше самого длинного тега - иначе теги встали бы
    // столбцом по одному в строке.
    expect(wrapper.vm.popoverWidth).toBeGreaterThan(widest * 7);
  });

  it('колонка расширилась и прятать стало нечего - список закрывается сам', async () => {
    wrapper = mountTags(ALL_FLAGS, 90);
    await wrapper.find('[data-testid="center-tags-more"]').trigger('click');
    expect(document.querySelector('[data-testid="center-tags-popover"]')).not.toBeNull();

    await wrapper.setProps({ availableWidth: 0 });

    expect(wrapper.vm.layout.hidden).toHaveLength(0);
    expect(document.querySelector('[data-testid="center-tags-popover"]')).toBeNull();
  });
});
