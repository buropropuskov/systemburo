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
    const wrapper = mountMark({ attachment_id: 5 });
    const btn = wrapper.find(BTN);
    expect(btn.exists()).toBe(true);
    expect(btn.attributes('disabled')).toBeUndefined();
    expect(btn.text()).toBe('Отметить как исполненное');
  });

  // Отсчёт идёт от числа секунд с сервера: по разнице с часами браузера кнопка
  // показывала «повтор через 5:02» при окне в пять минут.
  it('свежая отметка с бэка сразу блокирует кнопку и показывает отсчёт', () => {
    const wrapper = mountMark({
      attachment_id: 5,
      execution_marks: { today_count: 1, recent: [], seconds_left: 270 },
    });
    const btn = wrapper.find(BTN);
    expect(btn.attributes('disabled')).toBeDefined();
    expect(btn.text()).toBe('Отмечено, повтор через 4:30');
  });

  it('остаток никогда не превышает окна - часы браузера в счёте не участвуют', () => {
    const wrapper = mountMark({
      attachment_id: 5,
      execution_marks: { today_count: 1, recent: [], seconds_left: 300 },
    });
    expect(wrapper.find(BTN).text()).toBe('Отмечено, повтор через 5:00');
  });

  it('отметка старше окна кнопку не блокирует', () => {
    const wrapper = mountMark({ attachment_id: 5, execution_marks: { today_count: 0, recent: [], seconds_left: 0 } });
    const btn = wrapper.find(BTN);
    expect(btn.attributes('disabled')).toBeUndefined();
    expect(btn.text()).toBe('Отметить как исполненное');
  });

  it('клик отмечает вложение: запрос уходит по attachment_id, кнопка блокируется, приходит тост', async () => {
    markAccessibleAttachmentExecuted.mockResolvedValue({ seconds_left: 300 });
    const wrapper = mountMark({ attachment_id: 5 });

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
    const wrapper = mountMark({ attachment_id: 5 });

    await wrapper.find(BTN).trigger('click');
    await flushPromises();

    const btn = wrapper.find(BTN);
    expect(btn.attributes('disabled')).toBeUndefined();
    expect(btn.text()).toBe('Отметить как исполненное');
    expect(JSON.stringify(useDeletionsStore().items)).toContain('Уже отмечено недавно');
  });

  it('окно истекает без перезахода в деталь - кнопка сама разблокируется', async () => {
    const wrapper = mountMark({
      attachment_id: 5,
      execution_marks: { today_count: 1, recent: [], seconds_left: 3 },
    });
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
    [1, 'Сегодня 1 отметка'],
    [2, 'Сегодня 2 отметки'],
    [5, 'Сегодня 5 отметок'],
    [11, 'Сегодня 11 отметок'],
    [21, 'Сегодня 21 отметка'],
    [22, 'Сегодня 22 отметки'],
  ])('склонение для count=%i: %s', (count, expected) => {
    const wrapper = mountMark({
      attachment_id: 5,
      execution_marks: { today_count: count, recent: [], seconds_left: 0 },
    });
    expect(wrapper.find(SUMMARY).text()).toBe(expected);
  });

  // Блок под кнопкой занимал по две строки на отметку и раздувал карточку: теперь
  // одна строка, подробности - по клику.
  it('итог укладывается в одну строку и называет время последней отметки', () => {
    const wrapper = mountMark({
      attachment_id: 5,
      execution_marks: {
        today_count: 2,
        recent: [{ created_at: '2026-01-15T10:15:00.000Z', actor_name: 'Иванов П.С.' }],
        seconds_left: 0,
      },
    });
    expect(wrapper.find(SUMMARY).text()).toBe('Сегодня 2 отметки, последняя в 13:15');
    expect(wrapper.find(LIST).exists()).toBe(false);
  });

  it('список раскрывается по клику и показывает время по Москве с автором', async () => {
    const wrapper = mountMark({
      attachment_id: 5,
      execution_marks: {
        today_count: 2,
        recent: [
          { created_at: '2026-01-15T10:15:00.000Z', actor_name: 'Иванов П.С.' },
          { created_at: '2026-01-15T07:00:00.000Z', actor_name: null },
        ],
        seconds_left: 0,
      },
    });
    expect(wrapper.find(LIST).exists()).toBe(false);
    await wrapper.find(SUMMARY).trigger('click');

    const items = wrapper.findAll(`${LIST} .execution-mark__item`);
    expect(items).toHaveLength(2);
    expect(items[0].text()).toContain('13:15'); // UTC 10:15 + 3ч МСК
    expect(items[0].text()).toContain('Иванов П.С.');
    expect(items[1].text()).toContain('10:00'); // UTC 07:00 + 3ч МСК
  });

  it('успешная отметка эмитит marked - родитель перечитывает деталь', async () => {
    markAccessibleAttachmentExecuted.mockResolvedValue({ seconds_left: 300 });
    const wrapper = mountMark({ attachment_id: 5 });

    await wrapper.find(BTN).trigger('click');
    await flushPromises();

    expect(wrapper.emitted('marked')).toBeTruthy();
  });

  it('отказ бэка не эмитит marked - обновлять родителю нечего', async () => {
    markAccessibleAttachmentExecuted.mockRejectedValue(new Error('boom'));
    const wrapper = mountMark({ attachment_id: 5 });

    await wrapper.find(BTN).trigger('click');
    await flushPromises();

    expect(wrapper.emitted('marked')).toBeFalsy();
  });

  // Счётчик точный, а список ограничен сверху: без оговорки их расхождение читается
  // как потерянные отметки.
  it('обрезанный список объясняет себя', async () => {
    const wrapper = mountMark({
      attachment_id: 5,
      execution_marks: {
        today_count: 14,
        recent: Array(10).fill({ created_at: '2026-01-15T10:15:00.000Z', actor_name: 'Иванов И.И.' }),
      },
    });
    await wrapper.find(SUMMARY).trigger('click');

    expect(wrapper.find(SUMMARY).text()).toContain('14');
    expect(wrapper.find(LIST).text()).toContain('показаны последние 10');
  });

  it('полный список оговорки не несёт', async () => {
    const wrapper = mountMark({
      attachment_id: 5,
      execution_marks: {
        today_count: 2,
        recent: Array(2).fill({ created_at: '2026-01-15T10:15:00.000Z', actor_name: null }),
      },
    });
    await wrapper.find(SUMMARY).trigger('click');
    expect(wrapper.find(LIST).text()).not.toContain('показаны последние');
  });
});
