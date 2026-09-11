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
});
