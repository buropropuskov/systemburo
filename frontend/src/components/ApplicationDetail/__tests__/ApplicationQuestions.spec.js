import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/applications', () => ({
  getQuestions: vi.fn(),
  createQuestion: vi.fn(),
  createAnswer: vi.fn(),
  markQuestionRead: vi.fn(() => Promise.resolve()),
}));
import { getQuestions, markQuestionRead } from '@/api/applications';
import ApplicationQuestions from '../ApplicationQuestions.vue';

const QUESTIONS = [
  { id: 1, author_user_id: 5, author_name: 'Иванов', subject: 'Тема1', text: 'Т1', attachments: [], answers: [], created_at: '2026-07-01T10:00:00Z', is_new: false },
];

// Два новых топика для снимка новизны.
const QUESTIONS_NEW = [
  { id: 1, author_user_id: 5, author_name: 'Иванов', subject: 'Тема1', text: 'Т1', attachments: [], answers: [], created_at: '2026-07-01T10:00:00Z', is_new: true },
  { id: 2, author_user_id: 5, author_name: 'Иванов', subject: 'Тема2', text: 'Т2', attachments: [], answers: [], created_at: '2026-07-01T11:00:00Z', is_new: true },
];

function mountQ(props = {}) {
  return mount(ApplicationQuestions, {
    props: { applicationId: 42, attachments: [], currentUserId: 9, currentUserName: 'П', initiatorUserId: 5, canAsk: true, ...props },
    global: { stubs: { teleport: true, AskQuestionModal: true } },
  });
}

describe('ApplicationQuestions (#973)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getQuestions.mockReset();
    markQuestionRead.mockClear();
    localStorage.clear();
  });

  it('свёрнут по умолчанию и показывает темы обсуждения', async () => {
    getQuestions.mockResolvedValue(QUESTIONS);
    const wrapper = mountQ();
    await flushPromises();

    expect(getQuestions).toHaveBeenCalledWith(42);
    expect(wrapper.find('[data-testid="application-questions"]').classes()).toContain('collapsed');
    expect(wrapper.findAll('[data-testid="question-item"]')).toHaveLength(1);
    expect(wrapper.text()).toContain('Тема1');
  });

  it('пустой список: empty-state и кнопка «Начать обсуждение» при canAsk', async () => {
    getQuestions.mockResolvedValue([]);
    const wrapper = mountQ();
    await flushPromises();

    expect(wrapper.find('[data-testid="questions-empty"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="question-ask-button"]').exists()).toBe(true);
  });

  it('canAsk=false прячет кнопку «Начать обсуждение»', async () => {
    getQuestions.mockResolvedValue([]);
    const wrapper = mountQ({ canAsk: false });
    await flushPromises();

    expect(wrapper.find('[data-testid="question-ask-button"]').exists()).toBe(false);
  });

  it('тумблер заголовка разворачивает блок', async () => {
    getQuestions.mockResolvedValue([]);
    const wrapper = mountQ();
    await flushPromises();

    await wrapper.find('[data-testid="questions-toggle"]').trigger('click');
    expect(wrapper.find('[data-testid="application-questions"]').classes()).not.toContain('collapsed');
  });

  it('свёрнутость запоминается по каждой заявке отдельно', async () => {
    getQuestions.mockResolvedValue([]);
    const wrapper = mountQ({ applicationId: 100 });
    await flushPromises();

    // Развернуть заявку 100.
    await wrapper.find('[data-testid="questions-toggle"]').trigger('click');
    expect(wrapper.find('[data-testid="application-questions"]').classes()).not.toContain('collapsed');

    // Другая заявка 200 - своё состояние (дефолт свёрнута), не наследует от 100.
    await wrapper.setProps({ applicationId: 200 });
    await flushPromises();
    expect(wrapper.find('[data-testid="application-questions"]').classes()).toContain('collapsed');

    // Возврат на 100 - помнит развёрнутость.
    await wrapper.setProps({ applicationId: 100 });
    await flushPromises();
    expect(wrapper.find('[data-testid="application-questions"]').classes()).not.toContain('collapsed');
  });

  it('load() перезагружает обсуждение', async () => {
    getQuestions.mockResolvedValue([]);
    const wrapper = mountQ();
    await flushPromises();
    expect(wrapper.findAll('[data-testid="question-item"]')).toHaveLength(0);

    getQuestions.mockResolvedValue(QUESTIONS);
    await wrapper.vm.load();
    await flushPromises();
    expect(wrapper.findAll('[data-testid="question-item"]')).toHaveLength(1);
  });

  it('НЕ помечает прочитанным при открытии заявки (нет авто-markSeen)', async () => {
    getQuestions.mockResolvedValue(QUESTIONS_NEW);
    mountQ();
    await flushPromises();
    expect(markQuestionRead).not.toHaveBeenCalled();
  });

  it('индикатор заголовка виден при новых топиках (снимок is_new)', async () => {
    getQuestions.mockResolvedValue(QUESTIONS_NEW);
    const wrapper = mountQ();
    await flushPromises();
    expect(wrapper.find('[data-testid="questions-new-indicator"]').exists()).toBe(true);
    // Бейджи "Новое" на каждом новом топике.
    expect(wrapper.findAll('[data-testid="question-new-badge"]')).toHaveLength(2);
  });

  it('клик гасит бейдж топика сразу, индикатор - когда прочитаны все, и эмитит all-questions-read', async () => {
    getQuestions.mockResolvedValue(QUESTIONS_NEW);
    const wrapper = mountQ();
    await flushPromises();

    const subjects = wrapper.findAll('[data-testid="question-subject"]');
    expect(subjects).toHaveLength(2);
    expect(wrapper.findAll('[data-testid="question-new-badge"]')).toHaveLength(2);

    // Клик по первому -> его бейдж гаснет СРАЗУ (динамика), второй держится; индикатор горит.
    await subjects[0].trigger('click');
    expect(markQuestionRead).toHaveBeenCalledWith(42, 1);
    expect(wrapper.findAll('[data-testid="question-new-badge"]')).toHaveLength(1);
    expect(wrapper.find('[data-testid="questions-new-indicator"]').exists()).toBe(true);
    expect(wrapper.emitted('all-questions-read')).toBeFalsy();

    // Клик по второму -> все прочитаны: бейджи ушли, индикатор гаснет, эмит наверх (для списка).
    await subjects[1].trigger('click');
    expect(markQuestionRead).toHaveBeenCalledWith(42, 2);
    expect(wrapper.findAll('[data-testid="question-new-badge"]')).toHaveLength(0);
    expect(wrapper.find('[data-testid="questions-new-indicator"]').exists()).toBe(false);
    expect(wrapper.emitted('all-questions-read')).toBeTruthy();
    expect(wrapper.emitted('all-questions-read')[0]).toEqual([42]);
  });

  it('повторный клик по тому же топику не шлёт read дважды', async () => {
    getQuestions.mockResolvedValue(QUESTIONS_NEW);
    const wrapper = mountQ();
    await flushPromises();
    const subject = wrapper.findAll('[data-testid="question-subject"]')[0];
    await subject.trigger('click');
    await subject.trigger('click');
    expect(markQuestionRead).toHaveBeenCalledTimes(1);
  });
});
