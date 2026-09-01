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
 * 3. Обход общего входа. Рисунок из пункта 2 включает applyFilters, а половина
 *    фильтров звала загрузку сама: кнопка «Обновления», «на сегодня», поиск,
 *    сброс, вкладка архива, сортировка. Правка пункта 2 их не касалась, и
 *    владелец второй раз сообщил «анимации вообще нет». Замер на стенде после
 *    выката: на строках стоял app-row-leave-active, координата left не менялась.
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

const lines = source.split('\n');

// Заголовок метода, вотчера или хука: ровно 4-8 пробелов отступа, дальше имя и скобки.
// Ключевые слова отсеиваются - `if (...) {` формально подходит под ту же форму.
const MEMBER_HEADER = /^ {4,8}(?:async\s+)?([^\s(){}=;]+)\s*\([^)]*\)\s*\{\s*$/;
const CONTROL_KEYWORDS = new Set(['if', 'for', 'while', 'switch', 'catch', 'function']);

/** Имя члена компонента, внутри которого стоит строка. */
function enclosingMember(index) {
  for (let i = index; i >= 0; i -= 1) {
    const match = lines[i].match(MEMBER_HEADER);
    if (match && !CONTROL_KEYWORDS.has(match[1])) return match[1];
  }
  return null;
}

/** Тело члена компонента по имени - от заголовка до закрывающей скобки того же уровня. */
function memberBody(name) {
  const header = new RegExp(`^ {4,8}(?:async\\s+)?'?${name}'?\\s*\\(`);
  const start = lines.findIndex((line) => header.test(line));
  if (start < 0) return null;
  const indent = lines[start].match(/^ */)[0].length;
  const closing = new RegExp(`^ {${indent}}\\}`);
  const end = lines.findIndex((line, i) => i > start && closing.test(line));
  return lines.slice(start, end < 0 ? lines.length : end).join('\n');
}

/**
 * Кому позволено звать загрузку списка напрямую и почему.
 *
 * Всё остальное идёт через applyFilters: он один переключает набор переходов и
 * грузит тихо. Мимо него строка получает живой набор классов (уезжает вверх, а
 * не влево), а non-silent запрос накрывает список оверлеем «Обновление…».
 */
const DIRECT_FETCH_ALLOWED = {
  mounted: 'первая загрузка - анимировать нечего, набора ещё нет',
  applyFilters: 'сам общий вход',
  sortBy: 'перестановка своим рисунком, догрузка обёрнута в whileReplacing',
  refreshFromRealtime: 'приход по сигналу сервера - живая вставка, а не замена набора',
  handleConfirmationUpdate: 'дотягивание состояния после правки в карточке',
  handleApplicationUpdate: 'дотягивание состояния после правки в карточке',
  handleApplicationChanged: 'дотягивание состояния после правки в карточке',
};

/** Фильтры экрана: каждый обязан менять набор через общий вход. */
const FILTER_MEMBERS = [
  'archiveMode',
  'setMultiFilter',
  'onSearchInput',
  'clearMobileSearch',
  'toggleActiveToday',
  'toggleUnreadOnly',
  'toggleStatusUpdated',
  'resetFilters',
  'applyDateFilters',
  'clearDateRange',
];

/** Тело CSS-правила по селектору - от него до закрывающей скобки. */
function cssRule(selector) {
  const start = transitions.indexOf(selector);
  if (start < 0) return '';
  return transitions.slice(start, transitions.indexOf('}', start));
}

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
      source.includes(':name="rowTransition.transitionName.value"'),
      'TransitionGroup с постоянным именем прогоняет каскад появления и при смене фильтра',
    ).toBe(true);
    expect(source).not.toMatch(/<TransitionGroup[\s\S]{0,200}?\n\s+name="app-row"/);
  });

  it('смена фильтра идёт своим набором переходов и без оверлея', () => {
    const apply = source.slice(source.indexOf('        applyFilters() {'));
    const applyBody = apply.slice(0, apply.indexOf('\n        },'));

    expect(
      /rowTransition\.whileReplacing\(/.test(applyBody),
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

    expect(
      /position:\s*absolute/.test(cssRule('.app-row-filter-leave-active')),
      'уходящая строка обязана выходить из потока - иначе соседи ждут конца её анимации',
    ).toBe(true);

    // Появление без вертикального сдвига: он наложился бы на move соседей.
    const enterFrom = transitions.slice(transitions.indexOf('.app-row-filter-enter-from'));
    expect(
      /translateY/.test(enterFrom.slice(0, 120)),
      'у появления при фильтрации не должно быть сдвига - он читается как дрожание поверх move',
    ).toBe(false);
  });

  it('правила перехода ведут от контейнера, иначе их перебивает базовая строка', () => {
    const selectors = (transitions.match(/^[^\s@/*].*\{\s*$/gm) || []).map((l) => l.trim());
    expect(selectors.length, 'правила не разобрались - изменился формат файла').toBeGreaterThan(5);

    expect(
      selectors.filter((sel) => !sel.startsWith('.applications-list ')),
      'правило без контейнера имеет ту же специфичность, что базовое '
        + '.application-item { transition: background-color .2s } в самом вью. При равном '
        + 'счёте побеждает последний по порядку склейки, и строка получает '
        + 'transition-property: background-color - анимация пропадает целиком, класс при '
        + 'этом стоит на месте (замер на стенде: left менялся с 71 на 31 за один кадр)',
    ).toEqual([]);
  });

  it.each(['app-row', 'app-row-filter'])('%s: move объявлен выше leave', (set) => {
    const move = transitions.indexOf(`.${set}-move`);
    const leave = transitions.indexOf(`.${set}-leave-active`);
    expect(move, `${set}-move не найден`).toBeGreaterThan(-1);
    expect(leave, `${set}-leave-active не найден`).toBeGreaterThan(-1);

    expect(
      move < leave,
      `при массовом уходе Vue вешает на строку оба класса сразу: она сместилась `
        + `относительно прошлого кадра, значит попала в movedChildren. transition - `
        + `шорткат, и тот, что ниже, вытесняет свойства верхнего целиком. С move ниже `
        + `leave у уходящей остаётся только transform: строка едет влево, но гаснет `
        + `скачком (замер на стенде: opacity приняла 2 значения вместо 15)`,
    ).toBe(true);
  });

  it('оставшиеся едут только после того, как отсеянные уехали', () => {
    // Владелец: «СНАЧАЛА уезжают заявки, потом другие двигаются на свои места».
    // Без задержки обе фазы шли в один момент, и при коротком списке уход двух
    // заявок читался как «просто пропали».
    const moveDecl = cssRule('.applications-list .app-row-filter-move');
    const moveTimes = (moveDecl.match(/(\d*\.?\d+)s/g) || []).map(parseFloat);
    expect(moveTimes.length, 'у move нет задержки - фазы пойдут одновременно').toBe(2);

    const leaveRule = cssRule('.applications-list .app-row-filter-leave-active');
    const leaveTransform = leaveRule.slice(leaveRule.indexOf('transform'));
    const leaveDuration = parseFloat((leaveTransform.match(/(\d*\.?\d+)s/) || [])[1]);

    expect(
      moveTimes[1] >= leaveDuration,
      `задержка move (${moveTimes[1]}s) меньше времени ухода (${leaveDuration}s) - `
        + 'оставшиеся тронутся, когда отсеянные ещё едут, и фазы снова сольются',
    ).toBe(true);
  });

  it('пришедшие проявляются после того, как отсеянные уехали', () => {
    // При смене фильтра список перезапрашивается, и часть заявок приходит НОВЫМИ,
    // а не сдвигается: move им не достаётся, задержка нужна своя. Иначе они
    // проявляются на местах, ещё занятых уезжающими, и владелец видит, что
    // «ненужные заявки едут поверх вставших на их места».
    const enterDecl = cssRule('.applications-list .app-row-filter-enter-active');
    const enterTimes = (enterDecl.match(/(\d*\.?\d+)s/g) || []).map(parseFloat);
    expect(enterTimes.length, 'у появления нет задержки').toBe(2);

    const leaveRule = cssRule('.applications-list .app-row-filter-leave-active');
    const leaveTransform = leaveRule.slice(leaveRule.indexOf('transform'));
    const leaveDuration = parseFloat((leaveTransform.match(/(\d*\.?\d+)s/) || [])[1]);

    expect(
      enterTimes[1] >= leaveDuration,
      `задержка появления (${enterTimes[1]}s) меньше времени ухода (${leaveDuration}s)`,
    ).toBe(true);
  });

  it('уходящая строка лежит под остальными', () => {
    // position: absolute кладёт уходящую в слой выше статических соседей, и
    // z-index: 0 на ней этого не отменяет. Слой задаётся остающимся.
    const layer = cssRule('.applications-list > *');
    expect(
      /position:\s*relative/.test(layer) && /z-index:\s*[1-9]/.test(layer),
      'строки списка не подняты слоем - уезжающая будет проходить поверх них',
    ).toBe(true);
  });

  it('уход заметен на коротком списке', () => {
    const leaveTo = cssRule('.applications-list .app-row-filter-leave-to');
    const shift = Math.abs(parseFloat((leaveTo.match(/translateX\((-?\d+)px\)/) || [])[1]));

    expect(
      shift >= 100,
      `сдвиг ${shift}px: при четырёх заявках уход двух на сорок пикселей владелец `
        + 'прочитал как «просто пропали» - движению нужна амплитуда, глазу не за что '
        + 'зацепиться, когда соседи стоят',
    ).toBe(true);
  });

  it('режим замены держится весь цикл, а не до ближайшей отрисовки', () => {
    const composable = readFileSync(
      resolve(__dirname, '..', '..', 'composables', 'useRowTransition.js'),
      'utf8',
    );

    // Смена имени перехода - это ещё один рендер группы, а её onUpdated пересчитывает
    // FLIP и переписывает класс движения на текущий. Снятое сразу после патча имя
    // отбирало у строк фильтрационный класс вместе с его задержкой: на стенде они
    // трогались на 590мс с app-row-move, а разделители периодов не получали класса
    // вовсе и прыгали на 879мс, отставая от строк.
    expect(
      /setTimeout\(\s*\(\)\s*=>\s*\{[^}]*replacing\.value = false/.test(composable),
      'режим снимается сразу после отрисовки - класс движения перепишется на живой',
    ).toBe(true);
    expect(
      /clearTimeout\(release\)/.test(composable),
      'предыдущее снятие не отменяется - вторая смена фильтра погасит режим досрочно',
    ).toBe(true);

    const leaveMs = parseInt((composable.match(/LEAVE_MS = (\d+)/) || [])[1], 10);
    const leaveRule = cssRule('.applications-list .app-row-filter-leave-active');
    const leaveTransform = leaveRule.slice(leaveRule.indexOf('transform'));
    const leaveSeconds = parseFloat((leaveTransform.match(/(\d*\.?\d+)s/) || [])[1]);

    expect(
      leaveMs,
      'LEAVE_MS разошёлся с временем ухода в CSS: удержание высоты снимется не в такт',
    ).toBe(Math.round(leaveSeconds * 1000));

    expect(
      /REPLACE_MS = LEAVE_MS \* 2/.test(composable),
      'полный цикл считается не от времени ухода - при правке CSS числа разъедутся',
    ).toBe(true);
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

    // Микротика мало: он проходит раньше отрисовки, и Vue берёт живой набор классов
    // на строках, которые едут по фильтру. Проверено на стенде - уходящие получали
    // app-row-leave-active и уезжали вверх вместо влево.
    expect(
      /finally\s*\{\s*await nextTick\(\)/.test(composable),
      'перед снятием флага нет ожидания отрисовки - фильтрационный набор переходов не успеет примениться',
    ).toBe(true);
  });

  it('загрузку списка зовут только те, кому положено', () => {
    const callers = [...new Set(
      lines
        .map((line, i) => (line.includes('this.fetchApplications(') ? enclosingMember(i) : null))
        .filter(Boolean),
    )];

    expect(
      callers.filter((name) => !(name in DIRECT_FETCH_ALLOWED)),
      'смена набора мимо applyFilters: строки получат живой набор классов и уедут вверх, '
        + 'а не влево, non-silent запрос вдобавок накроет список оверлеем «Обновление…». '
        + 'Фильтру - applyFilters; если вызов правда не фильтрационный, впишите его в '
        + 'DIRECT_FETCH_ALLOWED с причиной',
    ).toEqual([]);

    expect(
      Object.keys(DIRECT_FETCH_ALLOWED).filter((name) => !callers.includes(name)),
      'разрешение выдано члену, который больше не грузит список - уберите строку, '
        + 'иначе она молча укроет будущий обход под тем же именем',
    ).toEqual([]);
  });

  it.each(FILTER_MEMBERS)('%s меняет набор через общий вход', (name) => {
    const body = memberBody(name);
    expect(body, `${name} не найден - переименовали? поправьте FILTER_MEMBERS`).toBeTruthy();

    expect(
      /this\.(applyFilters|setMultiFilter|onSearchInput)\(/.test(body),
      `${name} меняет фильтр, но не запрашивает набор через applyFilters - список останется прежним`,
    ).toBe(true);
  });

  it('перестановка по колонке идёт фильтрационным набором', () => {
    const body = memberBody('sortBy');
    expect(
      /rowTransition\.whileReplacing\(/.test(body),
      'клик по колонке двигает строки живым набором (0.3s ease) - один и тот же жест '
        + 'анимировался бы по-разному в зависимости от того, понадобилась ли догрузка',
    ).toBe(true);
    expect(
      /fetchApplications\(true\)/.test(body),
      'догрузка при сортировке не тихая - оверлей «Обновление…» скроет перестановку',
    ).toBe(true);
  });
});
