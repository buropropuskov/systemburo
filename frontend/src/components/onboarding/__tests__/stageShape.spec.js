import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { isFlushTarget, STAGE_PADDING, STAGE_RADIUS } from '../stageShape';

/**
 * Форма выреза подсветки (замечание владельца 20.08): панель поиска и рельс
 * навигации занимают экран во всю высоту, и общий зазор 10px уводил их вырез за
 * границу окна, а скругление 30px рисовало круглые углы у прямой панели.
 */
describe('isFlushTarget', () => {
  const size = { w: window.innerWidth, h: window.innerHeight };

  const rect = (x, y, w, h) => ({
    getBoundingClientRect: () => ({ x, y, width: w, height: h, left: x, top: y, right: x + w, bottom: y + h }),
  });

  beforeEach(() => {
    window.innerWidth = 1440;
    window.innerHeight = 900;
  });

  afterEach(() => {
    window.innerWidth = size.w;
    window.innerHeight = size.h;
  });

  it('панель во всю высоту экрана обводится встык', () => {
    // Панель сквозного поиска на 1440x900, замер со стенда.
    expect(isFlushTarget(rect(1020, 0, 420, 900))).toBe(true);
  });

  it('рельс навигации во всю высоту - тоже встык', () => {
    expect(isFlushTarget(rect(0, 0, 248, 900))).toBe(true);
  });

  it('мобильный drawer во всю ширину и высоту - встык', () => {
    window.innerWidth = 390;
    window.innerHeight = 844;
    expect(isFlushTarget(rect(0, 0, 390, 844))).toBe(true);
  });

  it('обычная цель внутри экрана сохраняет зазор и скругление', () => {
    // Кнопка колокольчика и карточка вложения - замеры со стенда.
    expect(isFlushTarget(rect(1186, 12, 35, 35))).toBe(false);
    expect(isFlushTarget(rect(82, 215, 1326, 118))).toBe(false);
  });

  it('высокая, но не достающая до низа панель остаётся со скруглением', () => {
    // Колонка Админки: 120..884 при высоте окна 900 - край не задет.
    expect(isFlushTarget(rect(50, 120, 263, 764))).toBe(false);
  });

  it('пустая цель и отсутствие цели - не встык (центр-модалка без подсветки)', () => {
    expect(isFlushTarget(rect(720, 450, 0, 0))).toBe(false);
    expect(isFlushTarget(null)).toBe(false);
    expect(isFlushTarget(undefined)).toBe(false);
  });
});

describe('пороги выреза', () => {
  it('значения по умолчанию - те же, что были зашиты в конфиг driver', () => {
    expect(STAGE_PADDING).toBe(10);
    expect(STAGE_RADIUS).toBe(30);
  });
});
