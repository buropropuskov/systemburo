import { mount } from '@vue/test-utils';
import {
  describe, it, expect, vi, beforeEach,
} from 'vitest';
import HintTooltip from '../HintTooltip.vue';

// Пузырёк телепортится в body с position:fixed внутри зазумленного <html> (проект
// масштабирует корень на мониторах >1440). Регресс-замок: updatePosition обязан
// приводить к layout-px И rect якоря, И innerWidth/innerHeight — если поделить
// только якорь, кламп считается по чужой ширине и подсказка вылезает за край
// экрана (ровно тот баг, ради которого её вообще выносили из ::after).
const zoom = { value: 1 };
vi.mock('@/utils/viewportScale', () => ({
  getViewportZoom: () => zoom.value,
}));

const BUBBLE_H = 100;

function mountHint(text = 'Подсказка') {
  return mount(HintTooltip, { props: { text, width: 260 }, attachTo: document.body });
}

/** Показывает подсказку с якорем в заданном месте и отдаёт вычисленный style. */
async function showAt(w, rect) {
  const anchor = w.vm.$refs.anchorEl ?? w.element;
  anchor.getBoundingClientRect = () => ({
    top: rect.top, bottom: rect.bottom, left: rect.left, width: rect.width, right: rect.left + rect.width, height: rect.bottom - rect.top,
  });
  await w.find('.hint-tooltip').trigger('focus');
  const bubble = document.querySelector('.hint-tooltip__bubble');
  // offsetHeight в jsdom всегда 0 — подменяем, чтобы проверить вертикальные клампы.
  Object.defineProperty(bubble, 'offsetHeight', { value: BUBBLE_H, configurable: true });
  await w.vm.$nextTick();
  // Пересчёт с уже известной высотой (в проде это делает тот же updatePosition).
  await w.find('.hint-tooltip').trigger('focus');
  return document.querySelector('.hint-tooltip__bubble').style;
}

describe('HintTooltip позиционирование под корневым zoom', () => {
  beforeEach(() => {
    zoom.value = 1;
    Object.defineProperty(window, 'innerWidth', { value: 1920, configurable: true });
    Object.defineProperty(window, 'innerHeight', { value: 1080, configurable: true });
    document.body.innerHTML = '';
  });

  it('zoom=1: пузырёк по центру якоря и над ним', async () => {
    const w = mountHint();
    const style = await showAt(w, { top: 500, bottom: 514, left: 800, width: 14 });

    // центр якоря 807 -> left = 807 - 130
    expect(parseFloat(style.left)).toBeCloseTo(677, 0);
    // над иконкой: 500 - 8 (зазор) - 100 (высота)
    expect(parseFloat(style.top)).toBeCloseTo(392, 0);
    expect(style.position).toBe('fixed');
  });

  it('zoom=1.2: кламп по правому краю считается в layout-px, пузырёк не вылезает за экран', async () => {
    zoom.value = 1.2;
    // Экран 1920 физических = 1600 layout-px. Якорь у правого края.
    const w = mountHint();
    const style = await showAt(w, { top: 600, bottom: 614, left: 1848, width: 14 }); // 1540 layout

    const left = parseFloat(style.left);
    const width = parseFloat(style.width);
    // Правый край пузырька обязан остаться внутри ЛЕЙАУТНОЙ ширины (1600), а не 1920.
    expect(left + width).toBeLessThanOrEqual(1600 - 8 + 0.5);
    expect(left).toBeGreaterThanOrEqual(8);
  });

  it('сверху не влезает -> флип вниз', async () => {
    const w = mountHint();
    // Якорь у самого верха: над ним 100px пузырька не поместится.
    const style = await showAt(w, { top: 40, bottom: 54, left: 400, width: 14 });
    expect(parseFloat(style.top)).toBeCloseTo(62, 0); // bottom(54) + зазор(8)
  });

  it('флип вниз у нижнего края клампится по высоте вьюпорта', async () => {
    const w = mountHint();
    // Якорь внизу: сверху не влезает (top<h), снизу пузырёк уехал бы за 1080.
    const style = await showAt(w, { top: 60, bottom: 1040, left: 400, width: 14 });
    // не ниже, чем vh - h - EDGE = 1080 - 100 - 8
    expect(parseFloat(style.top)).toBeLessThanOrEqual(972);
  });
});
