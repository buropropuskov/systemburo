import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import SchedulePlaceWarningPanel from '../SchedulePlaceWarningPanel.vue';

// jsdom не реализует matchMedia - без мока useNarrowScreen выходит по гарду и
// isNarrow навсегда false, то есть мобильное поведение не проверяется.
function mockMatchMedia(matches) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

// #1183 polish: плавающая панель предупреждений - агрегирует группы мест, скрывается
// крестиком, возвращается при появлении новых предупреждений.

const scheduleGroup = (over = {}) => ({
  name: 'Ворота Маугли',
  free: null,
  windows: [],
  schedule: {
    presence: '13:00—14:00',
    days: [{ label: 'Пн 13.07', hours: ['10:00—12:00'], open: false }],
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
    expect(el.textContent).toContain('Предупреждение');
    expect(el.textContent).toContain('Ворота Маугли');
    expect(el.textContent).toContain('Вы указали');
    expect(el.textContent).toContain('13:00—14:00');
    expect(el.textContent).toContain('Пн 13.07');
    expect(el.textContent).toContain('10:00—12:00');
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

  it('два места с одинаковым именем, но разным id -> обе группы рендерятся (уникальный ключ transition-group)', () => {
    mountPanel([
      scheduleGroup({ id: 'place-1', name: 'Склад' }),
      scheduleGroup({ id: 'table-1', name: 'Склад' }),
    ]);
    expect(panel()).not.toBeNull();
    expect(document.querySelectorAll('[data-testid="schedule-warning-panel"] .warn-group')).toHaveLength(2);
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

  it('на десктопе содержимое раскрыто сразу, счётчика и шеврона в шапке нет', () => {
    mountPanel([scheduleGroup()]);
    expect(document.querySelector('.warn-panel__reveal').classList.contains('warn-panel__reveal--open')).toBe(true);
    expect(document.querySelector('[data-testid="schedule-warning-count"]')).toBeNull();
    expect(document.querySelector('.warn-panel__chevron')).toBeNull();
  });

  describe('на телефоне панель сворачиваемая (#1097 P9)', () => {
    let origMatchMedia;

    beforeEach(() => {
      origMatchMedia = window.matchMedia;
      mockMatchMedia(true);
    });

    afterEach(() => {
      window.matchMedia = origMatchMedia;
    });

    const revealOpen = () =>
      document.querySelector('.warn-panel__reveal').classList.contains('warn-panel__reveal--open');

    it('появляется свёрнутой плашкой со счётчиком, содержимое свёрнуто', async () => {
      // isNarrow выставляется в onMounted - DOM догоняет на nextTick
      const w = mountPanel([scheduleGroup(), scheduleGroup({ id: 'p2', name: 'Дебаркадер №1' })]);
      await w.vm.$nextTick();
      const head = document.querySelector('[data-testid="schedule-warning-head"]');
      expect(head.getAttribute('aria-expanded')).toBe('false');
      expect(document.querySelector('[data-testid="schedule-warning-count"]').textContent).toBe('2');
      expect(revealOpen()).toBe(false);
    });

    it('тап по плашке разворачивает и сворачивает обратно', async () => {
      const w = mountPanel([scheduleGroup()]);
      await w.vm.$nextTick();
      const head = document.querySelector('[data-testid="schedule-warning-head"]');
      expect(revealOpen()).toBe(false);

      await head.click();
      await w.vm.$nextTick();
      expect(head.getAttribute('aria-expanded')).toBe('true');
      expect(revealOpen()).toBe(true);

      await head.click();
      await w.vm.$nextTick();
      expect(head.getAttribute('aria-expanded')).toBe('false');
      expect(revealOpen()).toBe(false);
    });

    it('крестик скрывает свёрнутую плашку, не разворачивая её', async () => {
      const w = mountPanel([scheduleGroup()]);
      await document.querySelector('[data-testid="schedule-warning-close"]').click();
      await w.vm.$nextTick();
      expect(panel()).toBeNull();
    });
  });
});
