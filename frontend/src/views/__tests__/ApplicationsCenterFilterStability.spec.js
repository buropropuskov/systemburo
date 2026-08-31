import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';

/**
 * Смена фильтра не должна двигать шапку и мигать списком.
 *
 * Две жалобы владельца, обе про одно - интерфейс дёргается при переключении:
 *
 * 1. Размеры. Счётчики кнопок считались по ЗАГРУЖЕННЫМ строкам, а фильтр
 *    «Обновления» на сервере отдаёт только прочитанные заявки. Непрочитанных в
 *    выборке становилось ноль, подпись «Новые: 5» укорачивалась до «Новые»,
 *    кнопка теряла 13px, соседняя съезжала. Замер на стенде: 85 -> 72, сосед
 *    x=508 -> 495. Лечится источником: сервер считает по всему скоупу доступа и
 *    от фильтров экрана не зависит.
 *
 * 2. Анимация. Каскад появления строк рассчитан на живую вставку одной-двух
 *    заявок (#840). При смене фильтра меняется весь набор, и тот же каскад
 *    прогонял через прозрачность все тридцать строк разом - список мигал
 *    целиком. Замер на стенде: opacity первой строки 0 -> 0.56 -> 0.92 -> 1.
 *
 * Замок текстовый: поднимать вью целиком ради двух связей дороже, чем сверить,
 * что источник счётчиков серверный, а имя перехода гасится на время замены.
 */

const viewPath = resolve(__dirname, '..', 'ApplicationsCenter.vue');
const source = readFileSync(viewPath, 'utf8');

/** Тело вычисляемого свойства по имени. */
function computedBody(name) {
  const start = source.indexOf(`        ${name}() {`);
  if (start < 0) return null;
  const end = source.indexOf('\n        },', start);
  return source.slice(start, end);
}

describe('Центр заявок: смена фильтра не дёргает шапку и список', () => {
  it.each(['unreadCount', 'statusUpdateCount'])(
    '%s берётся с сервера, а не считается по загруженным строкам',
    (name) => {
      const body = computedBody(name);
      expect(body, `${name} не найден`).toBeTruthy();

      expect(
        /this\.headerCounters\./.test(body),
        `${name}: счётчик обязан браться из useHeaderCounters - иначе включённый фильтр обнуляет соседний счётчик и кнопка меняет ширину`,
      ).toBe(true);

      expect(
        /this\.applications\.filter/.test(body),
        `${name}: счёт по загруженным строкам зависит от фильтра и от пагинации`,
      ).toBe(false);
    },
  );

  it('счётчики обновляются вместе со списком', () => {
    expect(source).toMatch(/useHeaderCounters\(\)/);
    expect(
      source.includes('this.headerCounters.refresh();'),
      'счётчики подключены, но не обновляются - числа застынут на нуле',
    ).toBe(true);
  });

  it('имя перехода строк вычисляемое, а не зашитое в разметку', () => {
    expect(
      source.includes(':name="rowTransitionName"'),
      'TransitionGroup с постоянным именем прогоняет каскад появления и при смене фильтра',
    ).toBe(true);
    expect(source).not.toMatch(/<TransitionGroup[\s\S]{0,200}?\n\s+name="app-row"/);
  });

  it('каскад гасится на время замены набора', () => {
    const apply = source.slice(source.indexOf('        applyFilters() {'));
    const applyBody = apply.slice(0, apply.indexOf('\n        },'));
    expect(
      /whileReplacingRows\(/.test(applyBody),
      'applyFilters не гасит каскад - смена фильтра снова будет мигать всем списком',
    ).toBe(true);
  });

  it('composable перехода возвращает каскад после замены', () => {
    const composable = readFileSync(
      resolve(__dirname, '..', '..', 'composables', 'useRowTransition.js'),
      'utf8',
    );
    expect(
      /replacing\.value\s*\?\s*''/.test(composable),
      'при замене набора имя перехода обязано быть пустым',
    ).toBe(true);
    expect(
      /finally\s*\{[\s\S]*?replacing\.value = false/.test(composable),
      'флаг снимается не в finally - при ошибке запроса каскад останется выключенным навсегда',
    ).toBe(true);
  });
});
