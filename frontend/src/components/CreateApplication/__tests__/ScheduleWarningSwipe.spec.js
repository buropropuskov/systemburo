import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import SchedulePlaceWarningPanel from '../SchedulePlaceWarningPanel.vue';

/**
 * Смахивание предупреждения вправо (волна 5 мобильного мокапа).
 *
 * jsdom проверяет логику жеста: порог, ось и то, что плашка действительно уходит.
 * Геометрия и ощущение от жеста проверяются в браузере - здесь их нет.
 */

// matchMedia в jsdom не реализован. Мок различает запросы: ширина - «телефон»,
// анимации - отдельным флагом. Общий мок «всё matches» включал бы и reduced
// motion, а тогда ветка со слайдом не проверялась бы никогда.
function mockMatchMedia({ narrow = true, reducedMotion = false } = {}) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches: query.includes('prefers-reduced-motion') ? reducedMotion : narrow,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

const GROUP = {
  id: 'place-1',
  name: 'Ворота Маугли',
  free: 'Дебаркадер №1: с 13:00 до 17:00 въезд через ПОСТ №72',
  windows: [],
  schedule: null,
};

const panel = () => document.querySelector('[data-testid="schedule-warning-panel"]');
const head = () => document.querySelector('[data-testid="schedule-warning-head"]');

/**
 * PointerEvent в jsdom нет - собираем MouseEvent того же типа и дописываем поля
 * указателя, которые читает обработчик.
 */
function pointer(el, type, { x = 0, y = 0, pointerType = 'touch' } = {}) {
  const event = new MouseEvent(type, { bubbles: true, cancelable: true, clientX: x, clientY: y });
  Object.defineProperty(event, 'pointerType', { value: pointerType });
  Object.defineProperty(event, 'pointerId', { value: 1 });
  Object.defineProperty(event, 'isPrimary', { value: true });
  el.dispatchEvent(event);
  return event;
}

/** Жест целиком: нажали, дошли до dx/dy в два шага, отпустили. */
async function swipe(wrapper, { dx, dy = 0, pointerType = 'touch', target = null }) {
  const el = target || panel();
  pointer(el, 'pointerdown', { x: 0, y: 0, pointerType });
  pointer(el, 'pointermove', { x: Math.round(dx / 2), y: Math.round(dy / 2), pointerType });
  pointer(el, 'pointermove', { x: dx, y: dy, pointerType });
  pointer(el, 'pointerup', { x: dx, y: dy, pointerType });
  await wrapper.vm.$nextTick();
}

let origMatchMedia;
let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  origMatchMedia = window.matchMedia;
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
  window.matchMedia = origMatchMedia;
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = '';
});

async function mountPanel(media = {}) {
  mockMatchMedia(media);
  wrapper = mount(SchedulePlaceWarningPanel, { props: { groups: [GROUP] }, attachTo: document.body });
  await wrapper.vm.$nextTick();
  return wrapper;
}

describe('смахивание предупреждения вправо', () => {
  it('жест дальше порога уводит плашку за край и снимает её', async () => {
    const w = await mountPanel();
    expect(panel()).not.toBeNull();

    await swipe(w, { dx: 120 });

    // Сначала доводим до края: снять сразу - плашка мигнёт на месте.
    expect(panel().style.transform).toBe('translateX(115%)');

    vi.advanceTimersByTime(220);
    await w.vm.$nextTick();
    expect(panel()).toBeNull();
  });

  it('жест короче порога возвращает плашку на место', async () => {
    const w = await mountPanel();

    await swipe(w, { dx: 40 });

    expect(panel()).not.toBeNull();
    expect(panel().style.transform).toBe('');

    vi.advanceTimersByTime(500);
    await w.vm.$nextTick();
    expect(panel()).not.toBeNull();
  });

  it('плашка тянется за пальцем, пока порог не пройден', async () => {
    const w = await mountPanel();
    const el = panel();

    pointer(el, 'pointerdown', { x: 0, y: 0 });
    pointer(el, 'pointermove', { x: 50, y: 0 });
    await w.vm.$nextTick();

    expect(panel().style.transform).toBe('translateX(50px)');
    expect(panel().classList.contains('is-dragging')).toBe(true);

    pointer(el, 'pointerup', { x: 50, y: 0 });
    await w.vm.$nextTick();
    expect(panel().classList.contains('is-dragging')).toBe(false);
  });

  it('вертикальный жест не закрывает плашку и не двигает её', async () => {
    const w = await mountPanel();

    await swipe(w, { dx: 4, dy: 140 });

    expect(panel()).not.toBeNull();
    expect(panel().style.transform).toBe('');

    vi.advanceTimersByTime(500);
    await w.vm.$nextTick();
    expect(panel()).not.toBeNull();
  });

  it('жест влево плашку не двигает', async () => {
    const w = await mountPanel();

    await swipe(w, { dx: -140 });

    expect(panel()).not.toBeNull();
    expect(panel().style.transform).toBe('');
  });

  it('при выключенных анимациях плашка снимается сразу, без слайда', async () => {
    const w = await mountPanel({ reducedMotion: true });

    await swipe(w, { dx: 120 });

    expect(panel()).toBeNull();
  });

  it('мышь в теле панели не таскает её - там выделяют текст', async () => {
    const w = await mountPanel();
    const body = document.querySelector('.warn-panel__body');
    expect(body).not.toBeNull();

    await swipe(w, { dx: 140, pointerType: 'mouse', target: body });

    expect(panel()).not.toBeNull();
    expect(panel().style.transform).toBe('');
  });

  it('мышью плашка тянется за шапку', async () => {
    const w = await mountPanel();

    await swipe(w, { dx: 140, pointerType: 'mouse', target: head() });
    vi.advanceTimersByTime(220);
    await w.vm.$nextTick();

    expect(panel()).toBeNull();
  });

  it('после жеста хвостовой клик по шапке не разворачивает плашку', async () => {
    const w = await mountPanel();
    expect(head().getAttribute('aria-expanded')).toBe('false');

    await swipe(w, { dx: 40, target: head() });
    head().click();
    await w.vm.$nextTick();

    expect(head().getAttribute('aria-expanded')).toBe('false');

    // Следующий тап - обычный: подавление одноразовое.
    head().click();
    await w.vm.$nextTick();
    expect(head().getAttribute('aria-expanded')).toBe('true');
  });

  it('крестик остаётся запасным путём', async () => {
    const w = await mountPanel();

    document.querySelector('[data-testid="schedule-warning-close"]').click();
    await w.vm.$nextTick();

    expect(panel()).toBeNull();
  });
});
