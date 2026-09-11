import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const markAccessibleAttachmentExecuted = vi.fn();
vi.mock('@/api/applications', () => ({
  markAccessibleAttachmentExecuted: (...args) => markAccessibleAttachmentExecuted(...args),
}));

import AttachmentExecutionMark from '../AttachmentExecutionMark.vue';
import { useDeletionsStore } from '@/stores/deletions';

function mountMark(attachment) {
  return mount(AttachmentExecutionMark, { props: { attachment } });
}

const BTN = '[data-testid="aa-mark-executed"]';

describe('AttachmentExecutionMark (#2446)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    markAccessibleAttachmentExecuted.mockReset();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('без вложения кнопку не рисует', () => {
    const wrapper = mountMark(null);
    expect(wrapper.find(BTN).exists()).toBe(false);
  });

  it('вложение без отметки - кнопка активна и предлагает отметить', () => {
    const wrapper = mountMark({ attachment_id: 5, execution_marked_until: null });
    const btn = wrapper.find(BTN);
    expect(btn.exists()).toBe(true);
    expect(btn.attributes('disabled')).toBeUndefined();
    expect(btn.text()).toBe('Отметить как исполненное');
  });

  it('свежая отметка с бэка сразу блокирует кнопку и показывает отсчёт', () => {
    const until = new Date(Date.now() + 4 * 60_000 + 30_000).toISOString();
    const wrapper = mountMark({ attachment_id: 5, execution_marked_until: until });
    const btn = wrapper.find(BTN);
    expect(btn.attributes('disabled')).toBeDefined();
    expect(btn.text()).toMatch(/^Отмечено, повтор через 4:3\d$/);
  });

  it('отметка старше окна кнопку не блокирует', () => {
    const until = new Date(Date.now() - 60_000).toISOString();
    const wrapper = mountMark({ attachment_id: 5, execution_marked_until: until });
    const btn = wrapper.find(BTN);
    expect(btn.attributes('disabled')).toBeUndefined();
    expect(btn.text()).toBe('Отметить как исполненное');
  });

  it('клик отмечает вложение: запрос уходит по attachment_id, кнопка блокируется, приходит тост', async () => {
    const until = new Date(Date.now() + 5 * 60_000).toISOString();
    markAccessibleAttachmentExecuted.mockResolvedValue({ execution_marked_until: until });
    const wrapper = mountMark({ attachment_id: 5, execution_marked_until: null });

    await wrapper.find(BTN).trigger('click');
    await flushPromises();

    expect(markAccessibleAttachmentExecuted).toHaveBeenCalledWith(5);
    const btn = wrapper.find(BTN);
    expect(btn.attributes('disabled')).toBeDefined();
    expect(btn.text()).toBe('Отмечено, повтор через 5:00');
    expect(JSON.stringify(useDeletionsStore().items)).toContain('исполненным');
  });

  it('отказ бэка (окно уже занято) не ломает кнопку и показывает причину', async () => {
    markAccessibleAttachmentExecuted.mockRejectedValue(new Error('Уже отмечено недавно, повторить можно позже'));
    const wrapper = mountMark({ attachment_id: 5, execution_marked_until: null });

    await wrapper.find(BTN).trigger('click');
    await flushPromises();

    const btn = wrapper.find(BTN);
    expect(btn.attributes('disabled')).toBeUndefined();
    expect(btn.text()).toBe('Отметить как исполненное');
    expect(JSON.stringify(useDeletionsStore().items)).toContain('Уже отмечено недавно');
  });

  it('окно истекает без перезахода в деталь - кнопка сама разблокируется', async () => {
    const until = new Date(Date.now() + 3000).toISOString();
    const wrapper = mountMark({ attachment_id: 5, execution_marked_until: until });
    expect(wrapper.find(BTN).attributes('disabled')).toBeDefined();

    await vi.advanceTimersByTimeAsync(4000);
    await wrapper.vm.$nextTick();

    expect(wrapper.find(BTN).attributes('disabled')).toBeUndefined();
    expect(wrapper.find(BTN).text()).toBe('Отметить как исполненное');
  });

  const SUMMARY = '[data-testid="aa-mark-summary"]';
  const LIST = '[data-testid="aa-mark-list"]';

  it('без execution_marks итоговая строка и список не рисуются', () => {
    const wrapper = mountMark({ attachment_id: 5 });
    expect(wrapper.find(SUMMARY).exists()).toBe(false);
    expect(wrapper.find(LIST).exists()).toBe(false);
  });

  it('today_count 0 скрывает и строку, и список - отмечать ещё не начинали', () => {
    const wrapper = mountMark({ attachment_id: 5, execution_marks: { today_count: 0, recent: [] } });
    expect(wrapper.find(SUMMARY).exists()).toBe(false);
    expect(wrapper.find(LIST).exists()).toBe(false);
  });

  it.each([
    [1, 'Сегодня отмечено 1 раз'],
    [2, 'Сегодня отмечено 2 раза'],
    [5, 'Сегодня отмечено 5 раз'],
    [11, 'Сегодня отмечено 11 раз'],
    [21, 'Сегодня отмечено 21 раз'],
    [22, 'Сегодня отмечено 22 раза'],
  ])('склонение "раз" для count=%i: %s', (count, expected) => {
    const wrapper = mountMark({
      attachment_id: 5,
      execution_marks: { today_count: count, recent: Array(count).fill({ created_at: '2026-01-15T10:15:00.000Z', actor_name: null }) },
    });
    expect(wrapper.find(SUMMARY).text()).toBe(expected);
  });

  it('список показывает время по Москве и автора отметки', () => {
    const wrapper = mountMark({
      attachment_id: 5,
      execution_marks: {
        today_count: 2,
        recent: [
          { created_at: '2026-01-15T10:15:00.000Z', actor_name: 'Иванов П.С.' },
          { created_at: '2026-01-15T07:00:00.000Z', actor_name: null },
        ],
      },
    });
    const items = wrapper.findAll(`${LIST} .execution-mark__item`);
    expect(items).toHaveLength(2);
    expect(items[0].text()).toContain('13:15'); // UTC 10:15 + 3ч МСК
    expect(items[0].text()).toContain('Иванов П.С.');
    expect(items[1].text()).toContain('10:00'); // UTC 07:00 + 3ч МСК
  });

  it('успешная отметка эмитит marked - родитель перечитывает деталь', async () => {
    markAccessibleAttachmentExecuted.mockResolvedValue({ execution_marked_until: new Date(Date.now() + 300000).toISOString() });
    const wrapper = mountMark({ attachment_id: 5, execution_marked_until: null });

    await wrapper.find(BTN).trigger('click');
    await flushPromises();

    expect(wrapper.emitted('marked')).toBeTruthy();
  });

  it('отказ бэка не эмитит marked - обновлять родителю нечего', async () => {
    markAccessibleAttachmentExecuted.mockRejectedValue(new Error('boom'));
    const wrapper = mountMark({ attachment_id: 5, execution_marked_until: null });

    await wrapper.find(BTN).trigger('click');
    await flushPromises();

    expect(wrapper.emitted('marked')).toBeFalsy();
  });
});
