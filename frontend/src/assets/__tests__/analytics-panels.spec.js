/**
 * Замок раскладки панелей аналитики (#2165 -> кольца уехали вправо).
 *
 * Холст Chart.js держит свой прежний размер, пока контейнер не заставит его
 * сжаться. Стоит дать карточке кольца центрирующий трек (place-items: center),
 * и она начинает сайзиться ПО холсту: карточка 280px, холст 300px, кольцо
 * вылезает за границу на 35px и уезжает с центра. Отсюда две проверки: карточка
 * задаёт ширину сама, а потомок обязан её принять.
 */
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const CSS = fs.readFileSync(
  path.join(path.resolve(__dirname, '../../..'), 'src/assets/analytics-panels.css'),
  'utf8',
);

/** Тело правила по селектору. */
function rule(selector) {
  const re = new RegExp(`${selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*\\{([^}]*)\\}`);
  const found = CSS.match(re);
  if (!found) throw new Error(`правило ${selector} не найдено`);
  return found[1];
}

describe('раскладка панелей аналитики', () => {
  it('карточка кольца не центрирует содержимое треком: она сайзилась бы по холсту', () => {
    const body = rule('.an-panel__chart');
    expect(body).not.toMatch(/place-items\s*:\s*center/);
    expect(body).not.toMatch(/place-content\s*:\s*center/);
    // Колонка: дети растягиваются по ширине карточки, центрирование - по вертикали.
    expect(body).toMatch(/flex-direction\s*:\s*column/);
    expect(body).toMatch(/justify-content\s*:\s*center/);
  });

  it('холст обязан принимать ширину карточки, а не диктовать свою', () => {
    const body = rule('.an-panel__chart > *');
    expect(body).toMatch(/width\s*:\s*100%/);
    expect(body).toMatch(/min-width\s*:\s*0/);
  });

  it('кольцо растёт по высоте карточки, но не сплющивается на коротком списке', () => {
    expect(rule('.an-panel__chart')).toMatch(/min-height\s*:\s*300px/);
    expect(rule('.an-panel__chart > *')).toMatch(/flex\s*:\s*1 1 auto/);
  });

  it('плитки прижаты к верху: иначе строки делят высоту карточки поровну', () => {
    expect(rule('.an-panel__tiles')).toMatch(/align-content\s*:\s*start/);
  });

  it('от шестой плитки список перестраивается: две колонки и строки вместо карточек', () => {
    // Порог, после которого столбец начинал тянуть страницу: девять типов
    // давали 830px против 326 у соседней панели.
    expect(rule('.an-panel__tiles:has(> :nth-child(6))'))
      .toMatch(/grid-template-columns\s*:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/);
    // Плитка становится строкой «подпись - число».
    const tile = rule('.an-panel .an-panel__tiles:has(> :nth-child(6)) > *');
    expect(tile).toMatch(/display\s*:\s*flex/);
    expect(tile).toMatch(/justify-content\s*:\s*space-between/);
    // По центру, а не по baseline: подпись из двух слов переносится, и число
    // вставало по её первой строке - в ряду часть чисел сидела вверху плитки.
    expect(tile).toMatch(/align-items\s*:\s*center/);
    expect(tile).not.toMatch(/align-items\s*:\s*baseline/);
    // Вес селектора: у плиток свои scoped-правила той же специфичности, и без
    // ведущего .an-panel отступы строки перебивались обратно на карточные.
    expect(CSS).toMatch(/\.an-panel\s+\.an-panel__tiles:has\(> :nth-child\(6\)\) > \*\s*\{/);
  });

  it('от тринадцатой - три колонки: двух снова мало', () => {
    expect(rule('.an-panel__tiles:has(> :nth-child(13))'))
      .toMatch(/grid-template-columns\s*:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/);
  });

  it('панель отдаёт кольцу фиксированный трек, а числам - остаток', () => {
    expect(rule('.an-panel')).toMatch(/grid-template-columns\s*:\s*minmax\(0,\s*280px\)\s+minmax\(0,\s*1fr\)/);
  });
});
