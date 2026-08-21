import { describe, it, expect } from 'vitest';
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

function mountTags(over = {}, availableWidth = 0) {
  return mount(ApplicationTags, { props: { application: application(over), availableWidth } });
}

describe('ApplicationTags', () => {
  it('без признаков не рисует ничего', () => {
    const wrapper = mount(ApplicationTags, {
      props: { application: { id: 1, status: 'Завершено', confirmation: 'Согласовано' } },
    });

    expect(wrapper.find('.application-tags').exists()).toBe(false);
  });

  it('держит testid, по которым его находят онбординг и тесты списка', () => {
    const wrapper = mountTags({
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
    const wrapper = mountTags({ blacklist_flags_count: 2 }, 90);
    const chs = wrapper.find('[data-testid="ob-center-blacklist-tag"]');

    expect(chs.classes()).toContain('rt-tag--mode-count');
    expect(chs.find('.rt-tag__text').text()).toBe('2');
    expect(chs.attributes('data-hint')).toContain('чёрный список');
  });

  it('на просторной колонке ЧС остаётся полной подписью', () => {
    const wrapper = mountTags({ blacklist_flags_count: 2 }, 168);
    const chs = wrapper.find('[data-testid="ob-center-blacklist-tag"]');

    expect(chs.classes()).toContain('rt-tag--mode-text');
    expect(chs.find('.rt-tag__text').text()).toBe('2 похожи на ЧС');
  });

  it('теги, которым не хватило места, собираются в счётчик с перечнем в подсказке', () => {
    const wrapper = mountTags({
      blacklist_flags_count: 3,
      has_roof_access: true,
      has_free_parking: true,
      sender_is_important: true,
      has_unseen_questions: true,
      has_open_supplement: true,
      has_files: true,
    }, 90);

    const more = wrapper.find('[data-testid="center-tags-more"]');
    expect(more.exists()).toBe(true);

    const shown = wrapper.findAll('.rt-tag').length - 1;
    const hiddenCount = Number(more.text().replace('+', ''));
    expect(shown + hiddenCount).toBe(8);
    expect(more.attributes('data-hint')).toContain('Крыша');
  });

  it('маркер новых вопросов виден и на свёрнутом теге', () => {
    const wrapper = mountTags({ blacklist_flags_count: 2, has_unseen_questions: true }, 90);
    const questions = wrapper.find('[data-testid="center-questions-badge-7"]');

    if (questions.exists()) {
      expect(questions.find('.rt-tag__q-dot').exists()).toBe(true);
    } else {
      // Тег ушёл под счётчик - тогда о нём говорит перечень в подсказке.
      expect(wrapper.find('[data-testid="center-tags-more"]').attributes('data-hint')).toContain('Вопросы');
    }
  });

  it('без ограничения ширины (мобильная карточка) все теги идут подписями', () => {
    const wrapper = mountTags({ blacklist_flags_count: 2, has_roof_access: true, has_free_parking: true }, 0);

    expect(wrapper.find('[data-testid="center-tags-more"]').exists()).toBe(false);
    expect(wrapper.findAll('.rt-tag--mode-text')).toHaveLength(4);
  });
});
