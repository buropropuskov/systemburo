import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import SchedulePlaceWarningPanel from '../SchedulePlaceWarningPanel.vue';
import { useUiStore } from '@/stores/ui';

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

beforeEach(() => {
  setActivePinia(createPinia());
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

    it('свёрнутая плашка помечена классом - рамка панели уходит в тон предупреждения', async () => {
      // В свёрнутом виде от панели видна ТОЛЬКО жёлтая шапка, и нейтральная рамка
      // вокруг неё читалась как чужая обводка (#1415).
      const w = mountPanel([scheduleGroup()]);
      await w.vm.$nextTick();
      expect(panel().classList.contains('warn-panel--collapsed')).toBe(true);

      await document.querySelector('[data-testid="schedule-warning-head"]').click();
      await w.vm.$nextTick();
      expect(panel().classList.contains('warn-panel--collapsed')).toBe(false);
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

/*
 * Обводку панели jsdom не считает (нет layout), поэтому замок читает CSS: шапка не
 * должна задавать рамку по ВСЕМ сторонам - по бокам она дублировала рамку панели,
 * а сверху срезалась её закруглением при overflow: hidden (#1415).
 */
describe('SchedulePlaceWarningPanel - обводка шапки', () => {
  const css = readFileSync(resolve(__dirname, '../SchedulePlaceWarningPanel.vue'), 'utf8');

  it('шапка задаёт только нижнюю границу', () => {
    const head = css.match(/\.warn-panel__head\s*\{([\s\S]*?)\}/);
    expect(head, 'нет правила .warn-panel__head').not.toBeNull();
    expect(head[1], 'рамка по всем сторонам режется закруглением панели')
      .not.toMatch(/^\s*border:\s/m);
    expect(head[1]).toMatch(/border-bottom:/);
  });

  it('тень панели берётся из токена темы', () => {
    const root = css.match(/\.warn-panel\s*\{([\s\S]*?)\}/);
    expect(root[1]).toMatch(/box-shadow:[^;]*var\(--shadow-drop\)/);
  });
});

/**
 * Слой панели задаётся инлайн-стилем, а не v-bind() в scoped CSS: панель уходит в
 * Teleport, переменная от v-bind до неё не доезжает, и в браузере z-index становится
 * auto - панель проваливалась под sticky-шапку списка Т/С и под ряд дат (обе z-index: 1).
 * В jsdom правило CSS не применяется, поэтому слой проверяется по атрибуту style.
 */
describe('SchedulePlaceWarningPanel - слой', () => {
  const src = readFileSync(resolve(__dirname, '../SchedulePlaceWarningPanel.vue'), 'utf8');

  it('z-index не задаётся через v-bind() в стилях', () => {
    const styles = src.slice(src.indexOf('<style'));
    expect(styles, 'v-bind() в scoped CSS телепортированной панели не доезжает')
      .not.toMatch(/z-index:\s*v-bind/);
  });

  it('слой по умолчанию доезжает до элемента', () => {
    mountPanel([scheduleGroup()]);
    expect(panel().style.zIndex).toBe('990');
  });

  it('слой из пропа перебивает дефолтный', () => {
    mount(SchedulePlaceWarningPanel, {
      props: { groups: [scheduleGroup()], zIndex: 1010 },
      attachTo: document.body,
    });
    expect(panel().style.zIndex).toBe('1010');
  });
});

describe('панель и онбординг-тур', () => {
  // Панель плавающая и во время тура ложилась на подсвеченный блок сроков (#1771).
  it('во время тура панель не рендерится', () => {
    useUiStore().tourActive = true;
    const wrapper = mount(SchedulePlaceWarningPanel, {
      props: { groups: [{ id: 1, name: 'Ворота 1', notes: ['Проверьте режим'] }] },
      global: { stubs: { Teleport: true } },
    });
    expect(wrapper.find('.warn-panel').exists()).toBe(false);
  });
});
