import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/applications', () => ({
  getQuestions: vi.fn(),
  createQuestion: vi.fn(),
  createAnswer: vi.fn(),
  markQuestionsSeen: vi.fn(() => Promise.resolve()),
}));
import { getQuestions } from '@/api/applications';
import ApplicationQuestions from '../ApplicationQuestions.vue';

const QUESTIONS = [
  { id: 1, author_user_id: 5, author_name: 'Иванов', subject: 'Тема1', text: 'Т1', attachments: [], answers: [], created_at: '2026-07-01T10:00:00Z' },
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
    localStorage.clear();
  });

  it('свёрнут по умолчанию и показывает вопросы', async () => {
    getQuestions.mockResolvedValue(QUESTIONS);
    const wrapper = mountQ();
    await flushPromises();

    expect(getQuestions).toHaveBeenCalledWith(42);
    expect(wrapper.find('[data-testid="application-questions"]').classes()).toContain('collapsed');
    expect(wrapper.findAll('[data-testid="question-item"]')).toHaveLength(1);
    expect(wrapper.text()).toContain('Тема1');
  });

  it('пустой список: empty-state и кнопка «Задать вопрос» при canAsk', async () => {
    getQuestions.mockResolvedValue([]);
    const wrapper = mountQ();
    await flushPromises();

    expect(wrapper.find('[data-testid="questions-empty"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="question-ask-button"]').exists()).toBe(true);
  });

  it('canAsk=false прячет кнопку «Задать вопрос»', async () => {
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

  it('load() перезагружает вопросы', async () => {
    getQuestions.mockResolvedValue([]);
    const wrapper = mountQ();
    await flushPromises();
    expect(wrapper.findAll('[data-testid="question-item"]')).toHaveLength(0);

    getQuestions.mockResolvedValue(QUESTIONS);
    await wrapper.vm.load();
    await flushPromises();
    expect(wrapper.findAll('[data-testid="question-item"]')).toHaveLength(1);
  });
});
