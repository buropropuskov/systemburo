import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

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
 *    Плюс список накрывал оверлей «Обновление…» - вторая причина моргания.
 *    Теперь у замены набора свой рисунок: отсеянные уезжают влево, оставшиеся
 *    подтягиваются на их места, пришедшие проявляются без сдвига.
 *
 * Замок текстовый: поднимать вью целиком ради двух связей дороже, чем сверить,
 * что источник счётчиков серверный, а имя перехода гасится на время замены.
 */

const viewPath = resolve(__dirname, '..', 'ApplicationsCenter.vue');
const source = readFileSync(viewPath, 'utf8');

// Переходы строк вынесены из вью отдельным файлом (гейт размеров: ApplicationsCenter.vue
// и так сверх порога). Правила подключаются @import-ом внутрь scoped-блока, поэтому
// остаются изолированными компонентом - меняется только место хранения.
const transitions = readFileSync(
  resolve(__dirname, '..', '..', 'assets', 'application-row-transitions.css'),
  'utf8',
);

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

  it('смена фильтра идёт своим набором переходов и без оверлея', () => {
    const apply = source.slice(source.indexOf('        applyFilters() {'));
    const applyBody = apply.slice(0, apply.indexOf('\n        },'));

    expect(
      /whileReplacingRows\(/.test(applyBody),
      'applyFilters не переключает набор переходов - список снова будет мигать целиком',
    ).toBe(true);
    expect(
      /fetchApplications\(true\)/.test(applyBody),
      'загрузка не тихая - оверлей «Обновление…» накроет список и даст моргание',
    ).toBe(true);
  });

  it('вью подключает вынесенные переходы', () => {
    expect(
      source.includes("@import '@/assets/application-row-transitions.css';"),
      'файл переходов не подключён - список останется вовсе без анимации',
    ).toBe(true);
  });

  it('отсеянные заявки уезжают влево, оставшиеся подтягиваются', () => {
    const leaveTo = transitions.slice(transitions.indexOf('.app-row-filter-leave-to'));
    expect(
      /translateX\(-\d+px\)/.test(leaveTo.slice(0, 160)),
      'уходящие строки обязаны уезжать влево (translateX), а не вверх',
    ).toBe(true);

    expect(
      transitions.includes('.app-row-filter-move'),
      'без move соседи перескочат на новые места рывком, а не подтянутся',
    ).toBe(true);

    const leaveActive = transitions.slice(transitions.indexOf('.app-row-filter-leave-active'));
    expect(
      /position:\s*absolute/.test(leaveActive.slice(0, 220)),
      'уходящая строка обязана выходить из потока - иначе соседи ждут конца её анимации',
    ).toBe(true);

    // Появление без вертикального сдвига: он наложился бы на move соседей.
    const enterFrom = transitions.slice(transitions.indexOf('.app-row-filter-enter-from'));
    expect(
      /translateY/.test(enterFrom.slice(0, 120)),
      'у появления при фильтрации не должно быть сдвига - он читается как дрожание поверх move',
    ).toBe(false);
  });

  it('composable возвращает живой набор переходов после замены', () => {
    const composable = readFileSync(
      resolve(__dirname, '..', '..', 'composables', 'useRowTransition.js'),
      'utf8',
    );
    expect(
      /replacing\.value\s*\?\s*replaceName\s*:\s*liveName/.test(composable),
      'режимы перепутаны или схлопнуты в один',
    ).toBe(true);
    expect(
      /finally\s*\{[\s\S]*?replacing\.value = false/.test(composable),
      'флаг снимается не в finally - при ошибке запроса режим останется включённым навсегда',
    ).toBe(true);
  });
});
