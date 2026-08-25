import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import DateFilter from '../DateFilter.vue';

/*
 * Контракт: календарь отдаёт результат в том виде, который заявлен пропом mode.
 * Родитель подписан только на события своего режима, поэтому выбор "не того вида"
 * (быстрый период в single-поле, "Сегодня" в range-поле) раньше уходил в никуда:
 * фильтр молча сбрасывался, а в поле оставался выбранный период.
 */
const mountFilter = (mode) => mount(DateFilter, {
  props: { mode },
  global: { stubs: { teleport: true } },
});

const lastEmit = (w, event) => {
  const calls = w.emitted(event) || [];
  return calls.length ? calls[calls.length - 1][0] : undefined;
};

describe('DateFilter - согласованность выхода с mode', () => {
  it('range: быстрый период в один день отдаётся диапазоном, а не одиночной датой', async () => {
    const w = mountFilter('range');
    await w.find('.date-field').trigger('click');

    await w.findAll('.quick-btn').find(b => b.text() === 'Сегодня').trigger('click');
    await w.find('.action-btn--apply').trigger('click');

    const start = lastEmit(w, 'update:dateRangeStart');
    const end = lastEmit(w, 'update:dateRangeEnd');
    expect(start).toBeInstanceOf(Date);
    expect(end).toBeInstanceOf(Date);
    expect(start.toDateString()).toBe(new Date().toDateString());
    expect(end.toDateString()).toBe(new Date().toDateString());
    expect(lastEmit(w, 'update:selectedDate')).toBeNull();
  });

  it('range: клик по одному дню в календаре даёт границы этого дня', async () => {
    const w = mountFilter('range');
    await w.find('.date-field').trigger('click');

    const day = w.findAll('.days-grid .day').find(d => d.text() === '15' && !d.classes().includes('other-month'));
    await day.trigger('click');
    await w.find('.action-btn--apply').trigger('click');

    const start = lastEmit(w, 'update:dateRangeStart');
    const end = lastEmit(w, 'update:dateRangeEnd');
    expect(start.getDate()).toBe(15);
    expect(end.getDate()).toBe(15);
    expect(start.getHours()).toBe(0);
    expect(end.getHours()).toBe(23);
  });

  it('single: показывает только периоды в один день и не даёт переключиться на диапазон', async () => {
    const w = mountFilter('single');
    await w.find('.date-field').trigger('click');

    const labels = w.findAll('.quick-btn').map(b => b.text());
    expect(labels).toContain('Сегодня');
    expect(labels).not.toContain('Этот месяц');
    expect(w.find('.calendar-mode-switch').exists()).toBe(false);
  });

  it('single: быстрый период отдаётся одиночной датой без границ диапазона', async () => {
    const w = mountFilter('single');
    await w.find('.date-field').trigger('click');

    await w.findAll('.quick-btn').find(b => b.text() === 'Вчера').trigger('click');
    await w.find('.action-btn--apply').trigger('click');

    const yesterday = new Date();
    yesterday.setDate(yesterday.getDate() - 1);
    expect(lastEmit(w, 'update:selectedDate').toDateString()).toBe(yesterday.toDateString());
    expect(lastEmit(w, 'update:dateRangeStart')).toBeNull();
    expect(lastEmit(w, 'update:dateRangeEnd')).toBeNull();
  });

  it('пустой выбор гасит обе формы результата', async () => {
    const w = mountFilter('range');
    await w.find('.date-field').trigger('click');
    await w.find('.action-btn--apply').trigger('click');

    expect(lastEmit(w, 'update:selectedDate')).toBeNull();
    expect(lastEmit(w, 'update:dateRangeStart')).toBeNull();
    expect(lastEmit(w, 'update:dateRangeEnd')).toBeNull();
    expect(w.emitted('apply')).toBeTruthy();
  });
});
