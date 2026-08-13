import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';

/**
 * Волна 6: шапки списков на телефоне сведены к одному виду.
 *
 * Владелец смотрит экраны подряд, поэтому разнобой между ними - дефект сам по себе:
 * «почему размер шрифта в шапке "Мои сотрудники" такой микроскопический» и «почему
 * padding по бокам у шапки меньше чем на других вкладках».
 *
 * Контракт, который стережёт этот файл:
 *   - кегль заголовка 18px - размер имени экрана в проекте (.center__title,
 *     .employeesview__title, .carsview__title); на мобилке заголовок страницы скрыт,
 *     и имя экрана несёт именно строка шапки списка;
 *   - боковой отступ шапки собран из слагаемых (отступ тела + рамка карточки + её
 *     внутренний отступ), чтобы заголовок стоял на одной вертикали с текстом карточек;
 *   - на очень узких (<=480) ни кегль, ни отступ не уменьшаются - именно уменьшение
 *     и дало «микроскопический» шрифт.
 *
 * Проверяем чтением SFC: jsdom не считает ни каскад, ни медиа-запросы, а сама
 * геометрия подтверждается замером в браузере.
 */

const VIEWS = path.resolve(__dirname, '..');

/** Кегль имени экрана: .center__title / .employeesview__title / .carsview__title. */
const TITLE_FONT_SIZE = 'font-size: 18px';

/** Отступ тела списка (8) + рамка карточки (1) + её внутренний отступ (14). */
const HEADER_PADDING = 'padding: 0 calc(8px + 1px + 14px)';

function read(name) {
  return readFileSync(path.join(VIEWS, name), 'utf8');
}

/** Блок мобильных правил: от @media 767.98 до следующего медиа-запроса. */
function mobileBlock(source) {
  const start = source.indexOf('@media (max-width: 767.98px)');
  expect(start, 'в файле нет блока @media (max-width: 767.98px)').toBeGreaterThan(-1);
  const rest = source.slice(start + '@media (max-width: 767.98px)'.length);
  const next = rest.indexOf('@media ');
  return next === -1 ? rest : rest.slice(0, next);
}

/** Блок правил для очень узких телефонов (<=480). */
function narrowBlock(source) {
  const start = source.indexOf('@media (max-width: 480px)');
  if (start === -1) return '';
  const rest = source.slice(start + '@media (max-width: 480px)'.length);
  const next = rest.indexOf('@media ');
  return next === -1 ? rest : rest.slice(0, next);
}

/** Тело правила по селектору, стоящему в начале строки. */
function rule(block, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = block.match(new RegExp(`^\\s*${escaped}\\s*\\{([^}]*)\\}`, 'm'));
  expect(match, `не нашёл правило ${selector}`).not.toBeNull();
  return match[1];
}

const SCREENS = [
  { name: 'Мои сотрудники', file: 'EmployeeView.vue', header: '.card-header', title: '.card-title' },
  { name: 'Мои автомобили', file: 'CarsView.vue', header: '.card-header', title: '.card-title' },
  { name: 'Доступные мне', file: 'AccessibleAttachmentsView.vue', header: '.management-header', title: '.management-title' },
];

describe.each(SCREENS)('$name - шапка списка на телефоне', (screen) => {
  const source = read(screen.file);
  const mobile = mobileBlock(source);

  it('заголовок набран кеглем имени экрана', () => {
    expect(rule(mobile, screen.title)).toContain(TITLE_FONT_SIZE);
  });

  it('заголовок стоит на одной вертикали с текстом карточек', () => {
    expect(rule(mobile, screen.header)).toContain(HEADER_PADDING);
  });

  it('шапка - одна строка 48px с многоточием вместо переноса', () => {
    const header = rule(mobile, screen.header);
    expect(header).toContain('height: 48px');
    expect(header).toContain('flex-wrap: nowrap');

    const title = rule(mobile, screen.title);
    expect(title).toContain('min-width: 0');
    expect(title).toContain('text-overflow: ellipsis');
  });

  it('на узких телефонах шапка не ужимается второй раз', () => {
    const narrow = narrowBlock(source);
    if (!narrow) return;
    // Уменьшенный кегль и отступ на <=480 - тот самый разнобой между 481 и 480.
    expect(narrow).not.toMatch(new RegExp(`\\${screen.title}\\s*\\{[^}]*font-size`));
    expect(narrow).not.toMatch(new RegExp(`\\${screen.header}\\s*\\{[^}]*padding`));
  });
});

describe('«Доступные мне» - порядок элементов шапки', () => {
  const source = read('AccessibleAttachmentsView.vue');
  const mobile = mobileBlock(source);

  /*
   * Претензия владельца: «Доступные мне + бейдж (количество) рядом, обновить с
   * другой стороны». Растущий заголовок (flex: 1 1 auto) отжимал счётчик вправо,
   * и тот вставал вплотную к «Обновить» вместо имени экрана.
   */
  it('счётчик прижат к заголовку, действия - к правому краю', () => {
    expect(rule(mobile, '.management-title')).toContain('flex: 0 1 auto');
    expect(rule(mobile, '.header-controls')).toContain('margin-left: auto');
    expect(rule(mobile, '.management-header')).toContain('justify-content: flex-start');
  });

  it('счётчик в разметке стоит между заголовком и действиями', () => {
    const header = source.slice(
      source.indexOf('class="management-header"'),
      source.indexOf('class="filters"'),
    );
    expect(header.indexOf('management-title')).toBeLessThan(header.indexOf('aa-count-badge'));
    expect(header.indexOf('aa-count-badge')).toBeLessThan(header.indexOf('header-controls'));
  });
});
