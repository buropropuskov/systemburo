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

  it('панель отдаёт кольцу фиксированный трек, а числам - остаток', () => {
    expect(rule('.an-panel')).toMatch(/grid-template-columns\s*:\s*minmax\(0,\s*280px\)\s+minmax\(0,\s*1fr\)/);
  });
});
