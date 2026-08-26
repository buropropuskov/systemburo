import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/applications', () => ({ createAnswer: vi.fn() }));
import { createAnswer } from '@/api/applications';
import ApplicationQuestionItem from '../ApplicationQuestionItem.vue';

const QUESTION = {
  id: 1,
  author_user_id: 5,
  author_name: 'Иванов И.И.',
  subject: 'Прицеп у фуры',
  text: 'Есть ли прицеп?',
  attachments: [{ id: 2, display_name: 'Автозаявка №1' }],
  created_at: '2026-07-01T10:00:00Z',
  answers: [
    { id: 11, author_user_id: 9, author_name: 'Петров П.П.', text: 'Да', created_at: '2026-07-01T11:00:00Z' },
  ],
};

function mountItem(props = {}) {
  return mount(ApplicationQuestionItem, {
    props: {
      question: JSON.parse(JSON.stringify(QUESTION)),
      applicationId: 42,
      currentUserId: 9,
      currentUserName: 'Петров П.П.',
      initiatorUserId: 9,
      ...props,
    },
  });
}

describe('ApplicationQuestionItem (#973)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    createAnswer.mockReset();
  });

  it('рендерит тему отдельно от автора, вложения и текст', () => {
    const wrapper = mountItem();
    expect(wrapper.find('.qi-subject').text()).toBe('Прицеп у фуры');
    expect(wrapper.text()).toContain('Иванов И.И.');
    expect(wrapper.text()).toContain('Автозаявка №1');
    expect(wrapper.text()).toContain('Есть ли прицеп?');
  });

  it('помечает инициатора заявки', () => {
    // ответ Петрова (author_user_id 9) = инициатор (initiatorUserId 9).
    const wrapper = mountItem();
    expect(wrapper.text()).toContain('Инициатор заявки');
  });

  it('тред свёрнут по умолчанию, тумблер раскрывает', async () => {
    const wrapper = mountItem();
    const toggle = wrapper.find('[data-testid="question-toggle-answers"]');
    expect(toggle.text()).toContain('Показать ответы (1)');
    await toggle.trigger('click');
    expect(wrapper.vm.expanded).toBe(true);
    expect(toggle.text()).toContain('Скрыть ответы (1)');
  });

  it('ответ добавляется оптимистично и подменяется сохранённым', async () => {
    createAnswer.mockResolvedValue({ id: 99, author_user_id: 9, author_name: 'Петров П.П.', text: 'Новый ответ', created_at: '2026-07-01T12:00:00Z' });
    const wrapper = mountItem();
    await wrapper.find('[data-testid="answer-input"]').setValue('Новый ответ');
    await wrapper.find('[data-testid="answer-send"]').trigger('click');
    await flushPromises();

    expect(createAnswer).toHaveBeenCalledWith(42, 1, { text: 'Новый ответ' });
    expect(wrapper.findAll('[data-testid="answer-item"]')).toHaveLength(2);
    expect(wrapper.emitted('answered')).toBeTruthy();
    expect(wrapper.vm.replyText).toBe('');
  });

  it('при ошибке ответ откатывается', async () => {
    createAnswer.mockRejectedValue(new Error('network'));
    const wrapper = mountItem();
    await wrapper.find('[data-testid="answer-input"]').setValue('Упадёт');
    await wrapper.find('[data-testid="answer-send"]').trigger('click');
    await flushPromises();

    expect(wrapper.findAll('[data-testid="answer-item"]')).toHaveLength(1);
    expect(wrapper.vm.replyText).toBe('Упадёт');
  });

  it('бейдж «Новое» виден при is-new и скрыт без него (#973)', () => {
    expect(mountItem({ isNew: false }).find('[data-testid="question-new-badge"]').exists()).toBe(false);
    expect(mountItem({ isNew: true }).find('[data-testid="question-new-badge"]').exists()).toBe(true);
  });

  it('клик по теме эмитит read и разворачивает тред (#973)', async () => {
    const wrapper = mountItem({ isNew: true });
    expect(wrapper.vm.expanded).toBe(false);
    await wrapper.find('[data-testid="question-subject"]').trigger('click');

    expect(wrapper.emitted('read')).toBeTruthy();
    expect(wrapper.emitted('read')[0]).toEqual([1]);
    expect(wrapper.vm.expanded).toBe(true);
  });

  it('разворачивание треда кнопкой тоже эмитит read (#973)', async () => {
    const wrapper = mountItem();
    await wrapper.find('[data-testid="question-toggle-answers"]').trigger('click');
    expect(wrapper.emitted('read')).toBeTruthy();
    expect(wrapper.emitted('read')[0]).toEqual([1]);
  });

  it('фокус по полю ответа тоже эмитит read (#973)', async () => {
    const wrapper = mountItem();
    await wrapper.find('[data-testid="answer-input"]').trigger('focus');
    expect(wrapper.emitted('read')).toBeTruthy();
    expect(wrapper.emitted('read')[0]).toEqual([1]);
  });
});
