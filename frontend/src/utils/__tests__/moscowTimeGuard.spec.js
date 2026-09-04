import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { resolve, join, relative } from 'node:path';

/**
 * Время в интерфейсе показывается московское и по часам сервера (#2298).
 *
 * Обычный тест это не стережёт: он видит результат одной функции, а беда
 * возвращается в новом файле - кто-то пишет свой форматтер и берёт зону машины,
 * потому что на его компьютере в Москве разницы не видно. Проверять приходится
 * по исходникам всего каталога.
 *
 * Запрещены ровно два приёма, у которых нет правильного применения:
 *
 *  1. Ручная прибавка трёх часов. Она была в новостях и в ленте журнала: время
 *     сдвигали, а показывали всё равно в зоне машины, и в Москве новость
 *     получала час на три больше настоящего.
 *  2. Показ текущего момента через локаль машины. Штамп «Дата формирования» в
 *     выгрузке уходит заказчику; сделанный по часам поста в другом поясе, он
 *     расходится с временем отметок внутри того же файла.
 *
 * Разбор строк даты вида new Date(год, месяц, день) не запрещён: там момент и
 * его показ живут в одной зоне, а приведение к Москве как раз увело бы день.
 */

const SRC = resolve(__dirname, '..', '..');
const SKIP_DIRS = new Set(['__tests__', 'node_modules', 'assets']);

/** Все .vue и .js каталога src, кроме тестов. */
function sourceFiles(dir = SRC, acc = []) {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) {
      if (!SKIP_DIRS.has(name)) sourceFiles(path, acc);
    } else if (name.endsWith('.vue') || name.endsWith('.js')) {
      acc.push(path);
    }
  }
  return acc;
}

const FILES = sourceFiles().map((path) => ({
  path: relative(SRC, path),
  text: readFileSync(path, 'utf8'),
}));

/** Файлы, где встретился запрещённый приём, с номерами строк. */
function offenders(pattern) {
  const found = [];
  for (const { path, text } of FILES) {
    const lines = text.split('\n');
    lines.forEach((line, i) => {
      if (pattern.test(line)) found.push(`${path}:${i + 1}`);
    });
  }
  return found;
}

describe('московское время не обходят стороной', () => {
  it('никто не прибавляет три часа руками', () => {
    expect(
      offenders(/3 \* 60 \* 60 \* 1000|\b10800000\b/),
      'сдвиг на три часа - это самодельная Москва: он не отменяет зону машины при '
        + 'показе и даёт двойное смещение. Нужен formatDateTime или formatMoscow* '
        + 'из utils/serverTime',
    ).toEqual([]);
  });

  it('текущий момент не показывают в зоне машины', () => {
    expect(
      offenders(/new Date\(\)\.toLocale|\bnow\.toLocale/),
      'штамп «сейчас» через локаль берёт часы и пояс компьютера. Для штампов '
        + 'выгрузок есть formatMoscowDateTime() из utils/serverTime',
    ).toEqual([]);
  });

  it('утилита времени на месте и зона в ней задана явно', () => {
    // Замок бесполезен, если сам источник правды подменили: проверка выше
    // прошла бы и на файле, где timeZone потеряли.
    const src = readFileSync(join(SRC, 'utils', 'serverTime.js'), 'utf8');
    expect(/const MSK = 'Europe\/Moscow'/.test(src)).toBe(true);
    expect(/timeZone: MSK/.test(src)).toBe(true);
  });
});
