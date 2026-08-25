import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { useEdgeSwipeOpen } from '../useEdgeSwipeOpen';

// #1097 W4.1: свайп вправо открывает боковое меню на мобилке. Ключевое требование
// пользователя - жест не должен отбирать системный «Назад» у телефонов с жестовой
// навигацией, поэтому первые пиксели у кромки экрана мы не трогаем вовсе.

let harness = null;
let onOpen = null;
let enabled = true;

function build(options = {}) {
  onOpen = vi.fn();
  enabled = true;
  harness = mount({
    template: '<div />',
    setup() {
      const swipe = useEdgeSwipeOpen(onOpen, { isEnabled: () => enabled, ...options });
      return { swipe };
    },
  }, { attachTo: document.body });
  return harness.vm.swipe;
}

function fire(type, x, y, target = document.body) {
  const event = new Event(type, { bubbles: true, cancelable: true });
  event.touches = x === null ? [] : [{ clientX: x, clientY: y }];
  target.dispatchEvent(event);
  return event;
}

function swipe(fromX, toX, { y = 300, toY = 300, target = document.body } = {}) {
  fire('touchstart', fromX, y, target);
  fire('touchmove', toX, toY, target);
  fire('touchend', toX, toY, target);
}

beforeEach(() => {
  document.body.innerHTML = '';
});

afterEach(() => {
  harness?.unmount();
  harness = null;
});

describe('useEdgeSwipeOpen', () => {
  it('свайп вправо дальше порога открывает панель', () => {
    build();
    swipe(60, 200);
    expect(onOpen).toHaveBeenCalledTimes(1);
  });

  it('жест от самой кромки экрана не наш - системный «Назад» остаётся рабочим', () => {
    const s = build({ deadZone: 24 });
    fire('touchstart', 10, 300);
    const move = fire('touchmove', 200, 300);
    fire('touchend', 200, 300);
    expect(onOpen).not.toHaveBeenCalled();
    expect(s.offset.value).toBe(0);
    // Не перехватили событие - браузер доигрывает свой жест сам.
    expect(move.defaultPrevented).toBe(false);
  });

  it('вертикальная прокрутка панель не тянет', () => {
    const s = build();
    fire('touchstart', 100, 200);
    fire('touchmove', 118, 320);
    expect(s.offset.value).toBe(0);
    expect(s.isDragging.value).toBe(false);
    fire('touchend', 118, 320);
    expect(onOpen).not.toHaveBeenCalled();
  });

  it('ось решается один раз: диагональ после вертикального старта панель не открывает', () => {
    build();
    fire('touchstart', 100, 200);
    fire('touchmove', 110, 300);
    fire('touchmove', 400, 320);
    fire('touchend', 400, 320);
    expect(onOpen).not.toHaveBeenCalled();
  });

  it('свайп влево панель не открывает', () => {
    build();
    swipe(200, 60);
    expect(onOpen).not.toHaveBeenCalled();
  });

  it('недотянутый до порога свайп возвращает панель на место', () => {
    const s = build({ threshold: 90 });
    fire('touchstart', 100, 300);
    fire('touchmove', 150, 300);
    expect(s.offset.value).toBe(50);
    expect(s.isDragging.value).toBe(true);
    fire('touchend', 150, 300);
    expect(onOpen).not.toHaveBeenCalled();
    expect(s.offset.value).toBe(0);
    expect(s.isDragging.value).toBe(false);
  });

  it('панель идёт за пальцем и не тянется дальше своей ширины', () => {
    const s = build({ width: 280 });
    fire('touchstart', 50, 300);
    fire('touchmove', 200, 300);
    expect(s.offset.value).toBe(150);
    fire('touchmove', 900, 300);
    expect(s.offset.value).toBe(280);
    fire('touchend', 900, 300);
  });

  it('свайп по полю ввода не перехватывается - там жест значит выделение', () => {
    build();
    const input = document.createElement('input');
    document.body.appendChild(input);
    swipe(60, 300, { target: input });
    expect(onOpen).not.toHaveBeenCalled();
  });

  it('свайп внутри горизонтально прокручиваемого блока листает его, а не меню', () => {
    build();
    const scroller = document.createElement('div');
    scroller.style.overflowX = 'auto';
    Object.defineProperty(scroller, 'scrollWidth', { value: 900 });
    Object.defineProperty(scroller, 'clientWidth', { value: 300 });
    const child = document.createElement('span');
    scroller.appendChild(child);
    document.body.appendChild(scroller);
    swipe(60, 300, { target: child });
    expect(onOpen).not.toHaveBeenCalled();
  });

  // Волна 6: «при свайпе предупреждения ещё открывается навигация» и «листал таблицу
  // вправо - постоянно открывалась навигация». Оба случая - один жест на двух хозяев.
  it('лента листается, даже когда её содержимое пока помещается', () => {
    build();
    const scroller = document.createElement('div');
    scroller.style.overflowX = 'auto';
    // Переполнения ещё нет: записей мало. Жест всё равно принадлежит ленте.
    Object.defineProperty(scroller, 'scrollWidth', { value: 300 });
    Object.defineProperty(scroller, 'clientWidth', { value: 300 });
    const child = document.createElement('span');
    scroller.appendChild(child);
    document.body.appendChild(scroller);
    swipe(60, 300, { target: child });
    expect(onOpen).not.toHaveBeenCalled();
  });

  it('элемент со своим горизонтальным жестом меню не отдаёт', () => {
    build();
    const panel = document.createElement('aside');
    panel.setAttribute('data-swipe-own', '');
    const inner = document.createElement('div');
    panel.appendChild(inner);
    document.body.appendChild(panel);
    swipe(60, 300, { target: inner });
    expect(onOpen).not.toHaveBeenCalled();
  });

  it('пока страница разъезжается вбок, свайп листает её, а не открывает меню', () => {
    build();
    const de = document.documentElement;
    const scrollW = Object.getOwnPropertyDescriptor(de, 'scrollWidth');
    Object.defineProperty(de, 'scrollWidth', { value: 900, configurable: true });
    Object.defineProperty(de, 'clientWidth', { value: 390, configurable: true });
    swipe(60, 300);
    expect(onOpen).not.toHaveBeenCalled();
    if (scrollW) Object.defineProperty(de, 'scrollWidth', scrollW);
    else delete de.scrollWidth;
  });

  it('при выключенном гейте (открытая панель или модалка) жест игнорируется', () => {
    build();
    enabled = false;
    swipe(60, 300);
    expect(onOpen).not.toHaveBeenCalled();
  });

  it('второй палец отменяет жест - не мешаем масштабированию', () => {
    const s = build();
    fire('touchstart', 60, 300);
    fire('touchmove', 200, 300);
    expect(s.offset.value).toBe(140);
    const pinch = new Event('touchmove', { bubbles: true, cancelable: true });
    pinch.touches = [{ clientX: 200, clientY: 300 }, { clientX: 260, clientY: 340 }];
    document.body.dispatchEvent(pinch);
    expect(s.offset.value).toBe(0);
    fire('touchend', 260, 300);
    expect(onOpen).not.toHaveBeenCalled();
  });

  it('после размонтирования слушатели сняты', () => {
    build();
    harness.unmount();
    harness = null;
    swipe(60, 300);
    expect(onOpen).not.toHaveBeenCalled();
  });
});
