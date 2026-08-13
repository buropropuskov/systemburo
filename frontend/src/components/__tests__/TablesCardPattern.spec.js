import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

/**
 * Замок на карточку строки в таблицах проходной (#1097 S9, талон машины - волна 5).
 *
 * Карточка собрана каскадом из двух слоёв - глобального responsive-tables.css и
 * scoped-блока компонента, - и ломается тихо: jsdom каскад и медиазапросы не считает,
 * поведенческий тест такого не видит. Стерегём поэтому сам контракт в исходнике.
 *
 * Что уже стреляло и чего эти проверки не дают повторить:
 * - доли столбцов заданы через `flex: N 0 0`, то есть с базисом 0. В строке базис и есть
 *   ширина, `width: 100%` при нём игнорируется - без `flex: 0 0 100%` все поля сбиваются
 *   обратно в одну строку по табличным долям;
 * - подпись рисуется `::before` у ячейки, и рядом с кнопкой прохода она дублировала саму
 *   кнопку («Въезд» подписью и «Въезд» кнопкой в одной строке);
 * - пунктир снизу у последнего поля висит оторванной чертой: последней в строке идёт
 *   колонка действий без data-label, и `[data-label]:last-child` до неё не достаёт.
 *
 * Все три таблицы поста собираются талоном (мокап docs/mockups/mobile-ux.html, экран
 * «Проходная»): пунктир между полями заменён одной линией отрыва, а статус и действия
 * переехали в подвал. Талон определяется наличием кнопок прохода, поэтому таблица
 * людей - тоже талон (волна 6: разнобой двух таблиц одного экрана владелец засчитал
 * как дефект), а FactTable в режиме people талона не собирает - кнопок прохода там
 * нет. Общая часть талона живёт в responsive-tables.css (часть 3) и стережётся
 * отдельным набором ниже; в компонентах остаётся раскладка полей, у каждой таблицы
 * своя. Различия описаны полями `tail`/`footCols`.
 *
 * Подвал переработан в волне 6 по претензии владельца («почему кнопки уехали вправо,
 * а не остались слева и не растянулись»): статус занимает свою строку, кнопки делят
 * следующую пополам - тем же базисом, что кнопки прохода наверху.
 */

const TABLES = [
  {
    file: 'CarsTable.vue',
    card: '.selected-table-card',
    row: '.selected-table-card .item-data.rt-row',
    head: '.items-header',
    tail: /\.(status|expand|actions)-col$/,
    footCols: ['.status-col', '.expand-col', '.actions-col'],
  },
  {
    file: 'PeopleTable.vue',
    card: '.selected-table-card',
    row: '.selected-table-card .item-data.rt-row',
    head: '.items-header',
    tail: /\.(status|expand|actions)-col$/,
    footCols: ['.status-col', '.expand-col', '.actions-col'],
  },
  {
    file: 'FactTable.vue',
    card: '.fact-table-card',
    row: '.fact-table-card .fact-row.rt-row',
    head: '.fact-header',
    tail: /\.(status|actions)-col$/,
    // «Подробнее» здесь нет: столбцы в карточке не прячутся, прятать нечего.
    footCols: ['.status-col', '.actions-col'],
  },
];

/** Ячейки, делящие строку пополам: пара прохода и кнопки подвала (статус - нет). */
const halfCols = ({ footCols }) => [
  '.entry-col',
  '.exit-col',
  ...footCols.filter((col) => col !== '.status-col'),
];

const MOBILE = '(max-width: 767.98px)';
const TABLET_UP = '(min-width: 768px)';
const SRC = path.resolve(__dirname, '../..');

const source = (file) =>
  fs.readFileSync(path.join(SRC, 'components', file), 'utf8')
    .replace(/\/\*[\s\S]*?\*\//g, '');

/** Тела всех блоков `@media <query>` с точно таким условием. */
function mediaBlocks(src, query) {
  const blocks = [];
  const re = /@media([^{]+)\{/g;
  let match;
  while ((match = re.exec(src)) !== null) {
    if (match[1].trim() !== query) continue;
    let depth = 0;
    for (let i = match.index + match[0].length - 1; i < src.length; i += 1) {
      if (src[i] === '{') depth += 1;
      else if (src[i] === '}') {
        depth -= 1;
        if (depth === 0) {
          blocks.push(src.slice(match.index + match[0].length, i));
          break;
        }
      }
    }
  }
  return blocks;
}

/** Объявления всех правил с таким селектором (в том числе в групповых). */
function declarationsFor(css, selector) {
  const out = [];
  const re = /([^{}]+)\{([^{}]*)\}/g;
  let match;
  while ((match = re.exec(css)) !== null) {
    const selectors = match[1].split(',').map((s) => s.trim());
    if (selectors.includes(selector)) out.push(match[2]);
  }
  return out;
}

/** Селекторы всех правил, где встречается такой фрагмент объявления. */
function selectorsWith(css, declaration) {
  const out = [];
  const re = /([^{}]+)\{([^{}]*)\}/g;
  let match;
  while ((match = re.exec(css)) !== null) {
    if (!match[2].includes(declaration)) continue;
    out.push(...match[1].split(',').map((s) => s.trim()));
  }
  return out;
}

describe.each(TABLES)('Карточка строки: $file', (table) => {
  const { file, card, row, head, tail, footCols } = table;
  const src = source(file);
  const mobile = mediaBlocks(src, MOBILE).join('\n');
  const cellRule = `${card} .rt-row > [data-label]`;

  it('карточка - строка с переносом, иначе кнопки прохода не встанут рядом', () => {
    const decls = declarationsFor(mobile, row).join('\n');
    expect(decls).toMatch(/flex-direction:\s*row\s*!important/);
    expect(decls).toMatch(/flex-wrap:\s*wrap\s*!important/);

    // Только на мобилке: на десктопе строка таблицы обязана остаться одной строкой.
    expect(selectorsWith(src, 'flex-wrap: wrap !important')).toEqual(
      selectorsWith(mobile, 'flex-wrap: wrap !important'),
    );
  });

  it('строку держит базис, и правило целит во все ячейки, а не только в подписанные', () => {
    // `width: 100%` при нулевом базисе не считается вовсе - ячейки делят одну строку,
    // и кнопка прохода вылезает из своей 14-пиксельной ячейки поверх соседей.
    const decls = declarationsFor(mobile, `${card} .rt-row > *`).join('\n');
    expect(decls).toMatch(/flex:\s*0\s+0\s+100%\s*!important/);

    // Ячейка без подписи (действия, «Подробнее») обязана попадать под то же правило.
    expect(declarationsFor(mobile, cellRule).join('\n'))
      .not.toMatch(/flex:\s*0\s+0\s+100%/);
  });

  it('значения выровнены влево', () => {
    const decls = declarationsFor(mobile, cellRule).join('\n');
    expect(decls).toMatch(/justify-content:\s*flex-start\s*!important/);
    expect(decls).toMatch(/text-align:\s*left\s*!important/);
  });

  it('подписи полей скрыты, кроме перечисленных исключений', () => {
    const hidden = declarationsFor(mobile, `${cellRule}::before`).join('\n');
    expect(hidden).toMatch(/display:\s*none\s*!important/);

    const shown = selectorsWith(mobile, 'display: block !important')
      .filter((s) => s.endsWith('::before'));
    expect(shown.length).toBeGreaterThan(0);
    shown.forEach((s) => expect(s.startsWith(`${card} .rt-row > .`)).toBe(true));
  });

  it('ячейки прохода делят строку и не несут пунктира', () => {
    for (const col of ['.entry-col', '.exit-col']) {
      const decls = declarationsFor(mobile, `${card} .rt-row > ${col}`).join('\n');
      // Базис ровно половина строки. Нулевой базис с ростом (`flex: 1 1 0`) здесь
      // не годится и однажды уже сломал талон: перенос во flex считается по базисам
      // ДО распределения свободного места, поэтому в первую строку набиралась ещё и
      // следующая ячейка, а обе кнопки схлопывались в 6px друг на друга.
      expect(decls).toMatch(/flex:\s*0\s+0\s+calc\(50%[^)]*\)\s*!important/);
      expect(decls).not.toMatch(/flex:\s*\d+\s+\d+\s+0(px)?\s*!important/);
      expect(decls).toMatch(/border-top:\s*none\s*!important/);
    }
  });

  it('пунктира снизу у полей нет', () => {
    // Последней в строке идёт колонка действий без data-label: глобальное
    // `[data-label]:last-child { border-bottom: none }` до неё не достаёт, и нижний
    // пунктир висел бы оторванной чертой над краем карточки.
    expect(declarationsFor(mobile, cellRule).join('\n'))
      .toMatch(/border-bottom:\s*none\s*!important/);
  });

  it('из «своя строка каждому» выходят пара прохода и кнопки подвала', () => {
    // Половина строки - и наверху, и в подвале: две кнопки одного размера читаются
    // как пара, а не как «одна большая и одна ужатая по слову».
    const expected = halfCols(table).map((col) => (
      col === '.entry-col' || col === '.exit-col'
        ? `${card} .rt-row > ${col}`
        : `${card} .rt-pass > ${col}`
    ));
    const exceptions = selectorsWith(mobile, 'flex: 0 0 calc(50% - 4px) !important');
    expect(exceptions.sort()).toEqual(expected.sort());
  });

  it('ширины по содержимому в карточке не осталось', () => {
    // `flex: 0 0 auto` в подвале и был причиной «кнопки уехали вправо»: ячейки шли по
    // слову, а свободное место автополе отдавало правому краю.
    const byContent = selectorsWith(mobile, 'flex: 0 0 auto !important');
    byContent.forEach((s) => expect(s).toMatch(tail));
    expect(byContent.filter((s) => footCols.some((col) => s.endsWith(col)))).toEqual([]);
  });

  it('кнопки подвала не прижаты к правому краю карточки', () => {
    const pushedRight = selectorsWith(mobile, 'margin-left: auto');
    footCols.forEach((col) => {
      expect(pushedRight.some((s) => s.endsWith(col))).toBe(false);
    });
  });

  it('кнопка прохода занимает свою половину строки', () => {
    const decls = declarationsFor(
      mobile,
      `${card} .rt-row > .entry-col .action-btn`,
    ).join('\n');
    expect(decls).toMatch(/width:\s*100%/);
  });

  it('обёртка полосы заголовков убрана из потока, а не только её внутренний ряд', () => {
    // rt-head-row прячет .header-row, но обёртка остаётся и рисует свой border-bottom
    // отдельной линией в 1px перед первой карточкой (эталон, §8).
    expect(declarationsFor(mobile, `${card} ${head}`).join('\n'))
      .toMatch(/display:\s*none/);

    // Закрепление полосы живёт в своём медиазапросе и обязано уцелеть.
    expect(declarationsFor(mediaBlocks(src, TABLET_UP).join('\n'), head).join('\n'))
      .toMatch(/position:\s*sticky/);
  });
});

/** Кнопка «Подробнее» есть там, где часть полей уходит из карточки. */
const WITH_CHEVRON = TABLES.filter(({ file }) => file !== 'FactTable.vue');

describe.each(WITH_CHEVRON)('Кнопка «Подробнее» в карточке: $file', ({ file, card }) => {
  const src = source(file);
  const mobile = mediaBlocks(src, MOBILE).join('\n');

  it('«Подробнее» и действия уходят в конец карточки, в этом порядке', () => {
    const order = (col) => {
      const decls = declarationsFor(mobile, `${card} .rt-pass > ${col}`).join('\n');
      const match = decls.match(/order:\s*(\d+)\s*!important/);
      return match ? Number(match[1]) : null;
    };

    const expand = order('.expand-col');
    const actions = order('.actions-col');
    expect(expand).not.toBeNull();
    expect(actions).not.toBeNull();
    // Разметочные order служебных столбцов (9997-9999) соседствуют с order статуса,
    // и шеврон оказывался посреди карточки - между именем и бейджем.
    expect(expand).toBeGreaterThan(9999);
    expect(actions).toBeGreaterThan(expand);
  });

  it('шеврон и корзина - пилюли с подписью, а не квадраты 44px', () => {
    // Владелец про прежний вид: «огромный квадрат со стрелочкой на пару вместе с
    // микроскопической иконкой корзины». Пилюля приходит классом из
    // responsive-tables.css, поэтому собственных квадратов в карточке быть не должно.
    expect(src).toMatch(/class="expand-btn rt-pass__act"/);
    expect(src).toMatch(/class="delete-btn rt-pass__act rt-pass__act--danger"/);

    const squares = selectorsWith(mobile, 'height: 44px');
    expect(squares).not.toContain(`${card} .expand-btn`);
    expect(squares).not.toContain('.delete-btn');
  });
});

const TALON = TABLES;

/**
 * Общая часть талона живёт в responsive-tables.css: линия отрыва, крупный номер,
 * приглушённая марка и пилюли подвала у обеих таблиц совпадают буква в букву, и
 * третьей копии этих правил быть не должно. Всё здесь обязано нести `!important` -
 * глобальный селектор проигрывает scoped-правилу потребителя и по специфичности, и
 * по порядку загрузки route-чанков.
 */
describe('Талон проходной: общая инфраструктура', () => {
  const css = fs.readFileSync(path.join(SRC, 'assets', 'responsive-tables.css'), 'utf8')
    .replace(/\/\*[\s\S]*?\*\//g, '');
  const mobile = mediaBlocks(css, MOBILE).join('\n');

  it('линия отрыва - псевдоэлемент строки, а не узел разметки', () => {
    const decls = declarationsFor(mobile, '.rt-pass::before').join('\n');
    // Своя строка: `.rt-row > *` псевдоэлемент не матчит, базис ему нужен собственный.
    // И это ШИРИНА КАРТОЧКИ, а не строки: отрицательные поля смещают полосу, но не
    // растягивают - у flex-элемента используемый размер равен базису. С базисом 100%
    // полоса выходила на 28px короче карточки и обрывалась справа (замер на 390:
    // 340 при 368 у карточки, правый край 351 против 379) - «разделитель не во всю
    // длину», претензия волны 6.
    expect(decls).toMatch(/flex:\s*0\s+0\s+calc\(100%\s*\+\s*28px\)\s*!important/);
    // Между кнопками прохода (order 0-1) и полями (от 10).
    expect(decls).toMatch(/order:\s*5\s*!important/);
    // Вырезы по краям - радиальные градиенты; без них остаётся просто пунктир.
    expect(decls.match(/radial-gradient/g)).toHaveLength(2);
    // Полоса тянется до краёв карточки поверх её padding (14px у .rt-row).
    expect(decls).toMatch(/margin:[^;]*-14px/);
  });

  it('поле талона не несёт пунктира сверху', () => {
    expect(declarationsFor(mobile, '.rt-pass > [data-label]').join('\n'))
      .toMatch(/border-top:\s*none\s*!important/);
  });

  it('номер крупный, и правило не убито переменной размера строки', () => {
    // У потребителей размер строки задан `.items-body .col { font-size: var(--table-font-size) }` -
    // правило и специфичнее глобального, и стоит ниже. Спасает только `!important`.
    const decls = declarationsFor(mobile, '.rt-pass > .rt-pass__plate').join('\n');
    expect(decls).toMatch(/font-size:\s*20px\s*!important/);
    expect(decls).toMatch(/font-variant-numeric:\s*tabular-nums\s*!important/);
  });

  it('кнопка подвала - пилюля 28px с зоной нажатия 44px во всю свою ячейку', () => {
    const decls = declarationsFor(mobile, '.rt-pass .rt-pass__act').join('\n');
    expect(decls).toMatch(/height:\s*28px\s*!important/);
    // Ширина от ячейки, а не от слова: ячейки подвала делят строку пополам, и пилюля
    // по содержимому жалась к левому краю своей половины.
    expect(decls).toMatch(/width:\s*100%\s*!important/);
    expect(decls).toMatch(/justify-content:\s*center\s*!important/);
    // Зона нажатия добирается невидимым ::before, иначе три элемента подвала не
    // помещаются в ширину карточки на 320.
    expect(declarationsFor(mobile, '.rt-pass .rt-pass__act::before').join('\n'))
      .toMatch(/inset:\s*-8px\s+-2px\s*!important/);
  });

  it('пилюля целит в свой класс, а не в кнопку потребителя', () => {
    // Под именем `.delete-btn` внутри строки живёт ещё и кнопка подменю корзины
    // (TableRowRemoveMenu): пилюлю она получает своим классом вместе с подписью,
    // а не тем, что правило целит в чужое имя.
    const selectors = selectorsWith(mobile, 'height: 28px !important');
    expect(selectors).not.toContain('.delete-btn');
    expect(selectors).not.toContain('.expand-btn');
  });

  it('подпись кнопки по умолчанию скрыта и включается только в талоне', () => {
    // Базовое правило - на голом классе (на десктопе подпись не нужна, там служебный
    // столбец узкий), включение - на потомке .rt-pass, то есть только в карточке.
    expect(declarationsFor(css, '.rt-pass__act-label').join('\n'))
      .toMatch(/display:\s*none/);
    expect(declarationsFor(mobile, '.rt-pass .rt-pass__act-label').join('\n'))
      .toMatch(/display:\s*inline\s*!important/);
  });
});

/**
 * Раскладка талона у каждой таблицы своя: набор столбцов разный, поэтому подвал и
 * список полей с подписями остаются в компонентах.
 */
describe.each(TALON)('Талон проходной: раскладка $file', ({ file, card, footCols }) => {
  const src = source(file);
  const mobile = mediaBlocks(src, MOBILE).join('\n');
  const ROW = `${card} .rt-pass`;

  it('строка помечена маркером талона, иначе общие правила мимо', () => {
    expect(src).toMatch(/class="[^"]*\brt-pass\b|'rt-pass':/);
    expect(src).toMatch(/\brt-pass__act\b/);
    expect(src).toMatch(/\brt-pass__act-label\b/);
  });

  it('у талона есть заголовок карточки', () => {
    // Крупная первая строка талона - номер с маркой у машин и ФИО у людей. Номер
    // набран моноширинными цифрами общим правилом, ФИО - раскладкой компонента,
    // поэтому маркер обязателен только там, где заголовок приходит из инфраструктуры.
    const plate = /\brt-pass__plate\b/.test(src) && /\brt-pass__mark\b/.test(src);
    const nameRow = /\blast-name-col\b/.test(src);
    expect(plate || nameRow).toBe(true);
  });

  it('пунктир между полями не рисуется', () => {
    const dashed = selectorsWith(mobile, 'border-top: 1px dashed');
    // Либо правила нет вовсе, либо оно явно исключает строку-талон.
    dashed.forEach((s) => expect(s).toMatch(/:not\(\.rt-pass\)/));
  });

  it('подвал: статус своей строкой, кнопки под ним пополам', () => {
    const order = (col) => {
      const decls = declarationsFor(mobile, `${ROW} > ${col}`).join('\n');
      return Number(decls.match(/order:\s*(\d+)\s*!important/)?.[1] ?? NaN);
    };
    // Разметочные order служебных столбцов соседствуют с order настраиваемых, и без
    // заведомо больших чисел статус с корзиной разъезжаются по середине карточки.
    const orders = footCols.map(order);
    expect(orders[0]).toBe(9999);
    orders.forEach((value, i) => {
      if (i > 0) expect(value).toBeGreaterThan(orders[i - 1]);
    });

    // Бейдж статуса занимает строку целиком: делить её с кнопками он не может -
    // на 320 половина остаётся 82px, и «Подробнее» в неё не влезает.
    expect(declarationsFor(mobile, `${ROW} > .status-col`).join('\n'))
      .toMatch(/flex:\s*0\s+0\s+100%\s*!important/);

    // Базовый `.col { overflow: hidden }` обрезал бы ::before пилюли, и зона нажатия
    // осталась бы 28px - палец мимо пилюли попадал бы в пустоту.
    const geometry = footCols
      .map((c) => declarationsFor(mobile, `${ROW} > ${c}`).join('\n'))
      .join('\n');
    expect(geometry).toMatch(/overflow:\s*visible/);
  });
});

/**
 * Карточка человека (волна 6). Владелец назвал состав дословно: ФИО одной строкой,
 * организация, срок действия. Первое - вопрос раскладки и живёт здесь; сам состав
 * полей проверяет TablesMobileCardFields.spec.js.
 */
describe('Карточка человека: ФИО одной строкой', () => {
  const mobile = mediaBlocks(source('PeopleTable.vue'), MOBILE).join('\n');
  const NAME_COLS = ['.last-name-col', '.first-name-col', '.middle-name-col'];

  it('ячейки имени идут по содержимому, а не каждая своей строкой', () => {
    const decls = NAME_COLS
      .map((col) => declarationsFor(mobile, `.selected-table-card .rt-row > ${col}`).join('\n'))
      .join('\n');
    // Базис auto, но сжатие разрешено: длинное ФИО переносится, а не распирает карточку.
    expect(decls).toMatch(/flex:\s*0\s+1\s+auto\s*!important/);
    expect(decls).toMatch(/font-weight:\s*700/);
    // Размер строки задан переменной ниже по файлу с той же специфичностью - без
    // `!important` заголовок карточки молча остаётся 14px (замер это и показал).
    expect(decls).toMatch(/font-size:\s*16px\s*!important/);
  });

  it('правило целит во все три ячейки имени', () => {
    NAME_COLS.forEach((col) => {
      expect(declarationsFor(mobile, `.selected-table-card .rt-row > ${col}`).join('\n'))
        .toMatch(/width:\s*auto\s*!important/);
    });
  });
});
