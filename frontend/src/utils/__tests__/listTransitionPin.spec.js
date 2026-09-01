import { describe, it, expect, vi } from 'vitest';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, resolve, relative } from 'node:path';

import { pinLeavingElement, holdParentHeight } from '../listTransition';

/**
 * Уходящие элементы списка обязаны закрепляться на своих местах.
 *
 * Приём «position: absolute на .*-leave-active» выводит уходящий элемент из
 * потока, чтобы соседи смыкались сразу. Для ОДНОГО элемента он верен: без
 * top/left тот встаёт на свою статическую позицию и остаётся, где был.
 *
 * При массовом уходе приём ломается молча. Вынимая из потока десятки строк
 * разом, браузер схлопывает их статические позиции к началу контейнера -
 * элементы ложатся стопкой наверху. Переходы отрабатывают честно (классы
 * применяются, transform меняется), но видит это только замер: снаружи список
 * меняется мгновенно.
 *
 * Так и вышло в Центре заявок: смена фильтра уводила 26 строк, анимация
 * проигрывалась в невидимой стопке, а владелец сообщил «анимации вообще нет».
 *
 * Замок держит оба конца: саму функцию закрепления и то, что каждая группа с
 * absolute в leave её подключила.
 */

const srcDir = resolve(__dirname, '..', '..');

/** Все .vue файлы фронтенда. */
function vueFiles(dir) {
  const out = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...vueFiles(full));
    } else if (entry.endsWith('.vue')) {
      out.push(full);
    }
  }
  return out;
}

/** Содержит ли файл leave-правило с position: absolute. */
function hasAbsoluteLeave(source) {
  const rules = source.match(/\.[\w-]*leave-active\s*\{[^}]*\}/g) || [];
  return rules.some((rule) => /position:\s*absolute/.test(rule));
}

describe('удержание высоты контейнера на время ухода', () => {
  /**
   * Уходящая строка выходит из потока сразу, и контейнер укорачивается тем же
   * кадром: при отсеве двух заявок из четырёх высота падала с 225 до 112 за
   * кадр, а подпись «Показано N» под списком прыгала вверх ровно тогда, когда
   * строки только начинали уезжать. Владелец описал это как «резко пропадают».
   */
  function parentWithHeight(height) {
    return {
      style: {},
      dataset: {},
      getBoundingClientRect: () => ({ height }),
    };
  }

  it('держит высоту заданное время и снимает после', () => {
    vi.useFakeTimers();
    const parent = parentWithHeight(225);

    holdParentHeight({ parentElement: parent }, 300);
    expect(parent.style.minHeight).toBe('225px');

    vi.advanceTimersByTime(299);
    expect(parent.style.minHeight, 'сняли раньше срока - подпись прыгнет посреди ухода').toBe('225px');

    vi.advanceTimersByTime(1);
    expect(parent.style.minHeight, 'высота осталась запертой - список не сомкнётся').toBe('');
    vi.useRealTimers();
  });

  it('ставится один раз на пачку уходящих', () => {
    vi.useFakeTimers();
    const parent = parentWithHeight(225);

    holdParentHeight({ parentElement: parent }, 300);
    // Второй уходящий приходит, когда первый уже вынут из потока: высота к этому
    // моменту меньше, и переустановка заперла бы список на укороченном значении.
    parent.getBoundingClientRect = () => ({ height: 112 });
    holdParentHeight({ parentElement: parent }, 300);

    expect(parent.style.minHeight).toBe('225px');
    vi.useRealTimers();
  });

  it('не падает на элементе без родителя', () => {
    expect(() => holdParentHeight({ parentElement: null }, 300)).not.toThrow();
  });
});

describe('закрепление уходящих элементов списка', () => {
  it('ставит координаты, размеры и снимает внешние отступы', () => {
    const parent = {
      scrollTop: 0,
      scrollLeft: 0,
      getBoundingClientRect: () => ({ top: 100, left: 50 }),
    };
    const el = {
      parentElement: parent,
      style: {},
      getBoundingClientRect: () => ({ top: 340, left: 50, width: 600, height: 72 }),
    };

    pinLeavingElement(el);

    // 340 - 100: строка обязана остаться на своём месте, а не уехать к началу списка.
    expect(el.style.top).toBe('240px');
    expect(el.style.left).toBe('0px');
    expect(el.style.width).toBe('600px');
    expect(el.style.height).toBe('72px');
    // Внешний отступ у элемента вне потока сдвинул бы его относительно места.
    expect(el.style.margin).toBe('0');
  });

  it('учитывает прокрутку контейнера', () => {
    const parent = {
      scrollTop: 200,
      scrollLeft: 0,
      getBoundingClientRect: () => ({ top: 100, left: 0 }),
    };
    const el = {
      parentElement: parent,
      style: {},
      getBoundingClientRect: () => ({ top: 150, left: 0, width: 300, height: 40 }),
    };

    pinLeavingElement(el);

    // Без учёта прокрутки строка закрепилась бы на 200px выше своего места.
    expect(el.style.top).toBe('250px');
  });

  it('не падает на элементе без родителя', () => {
    const el = { parentElement: null, style: {} };
    expect(() => pinLeavingElement(el)).not.toThrow();
  });

  it('каждая группа с absolute в leave подключает закрепление', () => {
    const offenders = [];

    for (const file of vueFiles(srcDir)) {
      const source = readFileSync(file, 'utf8');
      if (!/<transition-group|<TransitionGroup/i.test(source)) continue;
      if (!hasAbsoluteLeave(source)) continue;
      if (source.includes('pinLeavingElement')) continue;
      offenders.push(relative(srcDir, file));
    }

    expect(
      offenders,
      'уходящие строки выводятся из потока без закрепления позиции - при массовом уходе '
        + 'они сложатся стопкой в начале списка и анимации не будет видно; '
        + 'подключите @before-leave="pinLeavingElement" из utils/listTransition',
    ).toEqual([]);
  });
});
