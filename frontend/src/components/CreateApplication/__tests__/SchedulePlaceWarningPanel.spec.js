import { describe, it, expect, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import SchedulePlaceWarningPanel from '../SchedulePlaceWarningPanel.vue';

// #1183 polish: плавающая панель предупреждений - агрегирует группы мест, скрывается
// крестиком, возвращается при появлении новых предупреждений.

const scheduleGroup = (over = {}) => ({
  name: 'Ворота Маугли',
  free: null,
  windows: [],
  schedule: {
    presence: '13:00–14:00',
    days: [{ label: 'Пн 13.07', hours: '10:00–12:00', open: false }],
    anyClosed: true,
  },
  ...over,
});

const panel = () => document.querySelector('[data-testid="schedule-warning-panel"]');
const mountPanel = (groups) => mount(SchedulePlaceWarningPanel, { props: { groups }, attachTo: document.body });

afterEach(() => {
  document.body.innerHTML = '';
});

describe('SchedulePlaceWarningPanel', () => {
  it('пусто -> панель не рендерится', () => {
    mountPanel([]);
    expect(panel()).toBeNull();
  });

  it('группа без содержимого -> не рендерится', () => {
    mountPanel([{ name: 'X', free: null, windows: [], schedule: { anyClosed: false, days: [], presence: '' } }]);
    expect(panel()).toBeNull();
  });

  it('расписание вне графика -> показывает имя, режим дня и бейдж "вне графика"', () => {
    mountPanel([scheduleGroup()]);
    const el = panel();
    expect(el).not.toBeNull();
    expect(el.textContent).toContain('Ворота Маугли');
    expect(el.textContent).toContain('13:00–14:00');
    expect(el.textContent).toContain('Пн 13.07');
    expect(el.textContent).toContain('10:00–12:00');
    expect(el.textContent).toContain('вне графика');
  });

  it('свободный текст и окна показываются', () => {
    mountPanel([{ name: 'Пост', free: 'Только малогабарит', windows: ['Окно X'], schedule: null }]);
    const el = panel();
    expect(el.textContent).toContain('Только малогабарит');
    expect(el.textContent).toContain('Окно X');
  });

  it('крестик скрывает панель', async () => {
    const w = mountPanel([scheduleGroup()]);
    expect(panel()).not.toBeNull();
    await document.querySelector('[data-testid="schedule-warning-close"]').click();
    await w.vm.$nextTick();
    expect(panel()).toBeNull();
  });

  it('после скрытия новый состав предупреждений возвращает панель', async () => {
    const w = mountPanel([scheduleGroup()]);
    await document.querySelector('[data-testid="schedule-warning-close"]').click();
    await w.vm.$nextTick();
    expect(panel()).toBeNull();

    // добавилось новое место -> сигнатура изменилась -> панель снова видна
    await w.setProps({ groups: [scheduleGroup(), scheduleGroup({ name: 'Дебаркадер №1' })] });
    await w.vm.$nextTick();
    expect(panel()).not.toBeNull();
    expect(panel().textContent).toContain('Дебаркадер №1');
  });
});
