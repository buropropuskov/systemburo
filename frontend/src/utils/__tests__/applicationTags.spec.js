import { describe, it, expect } from 'vitest';
import {
  buildApplicationTags,
  layoutApplicationTags,
  tagWidth,
} from '@/utils/applicationTags';

// Реальные ширины колонки тегов в Центре: закреплённое нав-меню на 1280, обычная
// раскладка и широкий монитор (замер на staging).
const NARROW = 90;
const NORMAL = 132;
const WIDE = 168;

function application(over = {}) {
  return {
    id: 1,
    status: 'В обработке',
    confirmation: 'Согласование',
    sending_datetime: new Date(Date.now() - 6 * 86400000).toISOString(),
    ...over,
  };
}

/** Суммарная ширина строки тегов с зазорами - то, что реально займёт колонка. */
function rowWidth(layout) {
  const items = layout.visible.map((e) => tagWidth(e.tag, e.mode));
  if (layout.hidden.length) items.push(18 + 12);
  return items.reduce((a, b) => a + b, 0) + 4 * Math.max(0, items.length - 1);
}

describe('buildApplicationTags', () => {
  it('собирает теги в порядке приоритета: ЧС и срок ожидания впереди справочных', () => {
    const tags = buildApplicationTags(application({
      blacklist_flags_count: 2,
      has_roof_access: true,
      has_files: true,
      has_unseen_questions: true,
    }));

    expect(tags.map((t) => t.key)).toEqual(['chs', 'awaiting', 'questions', 'roof', 'files']);
  });

  it('у ЧС и срока есть короткая форма с числом, у справочных её нет', () => {
    const [chs, awaiting, roof] = buildApplicationTags(application({
      blacklist_flags_count: 2,
      has_roof_access: true,
    }));

    expect(chs.text).toBe('2 похожи на ЧС');
    expect(chs.countText).toBe('2');
    expect(awaiting.text).toBe('6 дн.');
    expect(awaiting.countText).toBe('6');
    expect(roof.countText).toBeNull();
  });

  it('заявка без признаков тегов не имеет', () => {
    expect(buildApplicationTags(application({ status: 'Завершено', confirmation: 'Согласовано' }))).toEqual([]);
  });
});

describe('layoutApplicationTags', () => {
  it('без ограничения по ширине показывает всё полным текстом (мобильная карточка)', () => {
    const tags = buildApplicationTags(application({ blacklist_flags_count: 2, has_roof_access: true }));
    const layout = layoutApplicationTags(tags, 0);

    expect(layout.hidden).toEqual([]);
    expect(layout.visible.map((e) => e.mode)).toEqual(['text', 'text', 'text']);
  });

  it('ЧС и срок ожидания вместе укладываются в обычную колонку: срок текстом, ЧС числом', () => {
    const tags = buildApplicationTags(application({ blacklist_flags_count: 2 }));
    const layout = layoutApplicationTags(tags, NORMAL);

    expect(layout.hidden).toEqual([]);
    expect(layout.visible.map((e) => `${e.tag.key}:${e.mode}`)).toEqual(['chs:count', 'awaiting:text']);
    expect(rowWidth(layout)).toBeLessThanOrEqual(NORMAL);
  });

  it('на широкой колонке ЧС возвращает себе подпись', () => {
    const tags = buildApplicationTags(application({ blacklist_flags_count: 2 }));
    const layout = layoutApplicationTags(tags, WIDE);

    expect(layout.visible[0].mode).toBe('text');
    expect(rowWidth(layout)).toBeLessThanOrEqual(WIDE);
  });

  it('в узкой колонке (нав-меню закреплено) оба тега сжимаются до числа', () => {
    const tags = buildApplicationTags(application({ blacklist_flags_count: 2 }));
    const layout = layoutApplicationTags(tags, NARROW);

    expect(layout.visible.map((e) => e.mode)).toEqual(['count', 'count']);
    expect(rowWidth(layout)).toBeLessThanOrEqual(NARROW);
  });

  it('справочные теги сворачиваются в иконку раньше, чем ЧС теряет число', () => {
    const tags = buildApplicationTags(application({
      blacklist_flags_count: 2,
      has_roof_access: true,
      has_free_parking: true,
    }));
    const layout = layoutApplicationTags(tags, NORMAL);
    const modes = Object.fromEntries(layout.visible.map((e) => [e.tag.key, e.mode]));

    expect(modes.chs).toBe('count');
    expect(modes.roof ?? 'hidden').not.toBe('text');
  });

  it('что не влезло - уходит в счётчик, ЧС остаётся на виду', () => {
    const tags = buildApplicationTags(application({
      blacklist_flags_count: 3,
      has_roof_access: true,
      has_free_parking: true,
      sender_is_important: true,
      has_unseen_questions: true,
      has_open_supplement: true,
      has_files: true,
    }));
    const layout = layoutApplicationTags(tags, NARROW);

    expect(tags).toHaveLength(8);
    expect(layout.hidden.length).toBeGreaterThan(0);
    expect(layout.visible[0].tag.key).toBe('chs');
    expect(layout.visible.length + layout.hidden.length).toBe(8);
    expect(rowWidth(layout)).toBeLessThanOrEqual(NARROW);
  });

  it('строка тегов не перерастает колонку ни на одном наборе признаков', () => {
    const flags = ['has_roof_access', 'has_free_parking', 'sender_is_important',
      'has_unseen_questions', 'has_open_supplement', 'has_files'];

    for (const width of [NARROW, 100, 120, NORMAL, 150, WIDE]) {
      // Полный перебор наборов справочных признаков поверх ЧС и срока ожидания:
      // именно комбинации, а не единичный кейс, ловят наезд на колонку действий.
      for (let mask = 0; mask < (1 << flags.length); mask++) {
        for (const chs of [0, 3]) {
          const over = { blacklist_flags_count: chs };
          flags.forEach((flag, i) => { if (mask & (1 << i)) over[flag] = true; });
          const tags = buildApplicationTags(application(over));
          if (!tags.length) continue;

          const layout = layoutApplicationTags(tags, width);
          expect(layout.visible.length).toBeGreaterThan(0);
          expect(layout.visible.length + layout.hidden.length).toBe(tags.length);
          // Один тег шире колонки ужать некуда - он сжимается сам (max-width: 100%),
          // проверяем случаи, где выбор раскладки вообще возможен.
          if (layout.visible.length + layout.hidden.length > 1) {
            expect(rowWidth(layout)).toBeLessThanOrEqual(width);
          }
        }
      }
    }
  });
});
