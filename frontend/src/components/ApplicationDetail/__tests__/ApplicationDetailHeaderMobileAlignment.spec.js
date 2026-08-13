import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

const DETAIL_SFC = readFileSync(resolve(__dirname, '../ApplicationDetail.vue'), 'utf8');
const ACTION_BAR_SFC = readFileSync(resolve(__dirname, '../ApplicationActionBar.vue'), 'utf8');

/** Вырезает /* ... *\/ комментарии - иначе пояснение в стиле "было flex: 1 1 auto"
 *  ложно матчится как действующее правило. */
function stripComments(css) {
  return css.replace(/\/\*[\s\S]*?\*\//g, ' ');
}

/** Тело ПЕРВОГО правила для селектора, без учёта переносов и комментариев (см.
 *  паттерн в ApplicationDetailHeaderCrest.spec.js / ApplicationAttachmentSupplementMarks.spec.js). */
function rule(src, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const found = src.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`));
  return found ? stripComments(found[1]).replace(/\s+/g, ' ').trim() : null;
}

/** Тела ВСЕХ правил для селектора по порядку - файл объявляет один и тот же класс
 *  дважды (база + @media override), и нужно проверить именно мобильный. */
function ruleAll(src, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const re = new RegExp(`${escaped}\\s*\\{([^}]*)\\}`, 'g');
  return [...src.matchAll(re)].map((m) => stripComments(m[1]).replace(/\s+/g, ' ').trim());
}

// Регрессия волны 7 (#1097 w8, дословно владельца): "Зачем ты отцентровал бейдж
// дополнения, а "Отозвать из работы"/"В работе" куда-то вправо". Причина - обёртка
// .detail-header-actions получила flex-grow (флекс 1 1 auto) и растягивалась на всю
// ширину шапки; justify-content:flex-end внутри такой широкой рамки на мобилке
// подвешивал контент у правого края с пустотой слева (замерено: 98px). Крестик при
// этом остаётся починенным волной 7 - те правила не трогали (см.
// ApplicationDetailHeaderCrest.spec.js).
describe('ApplicationDetail - шапка не растягивает .detail-header-actions на всю ширину (#1097 w8)', () => {
  it('обёртка бейджа/action-bar НЕ растёт (нет flex-grow) - хугает свой контент, как до волны 7', () => {
    const body = rule(DETAIL_SFC, '.detail-header-actions');
    expect(body).not.toMatch(/flex:\s*1\s+1\s+auto/);
    expect(body).not.toMatch(/flex-grow:\s*[1-9]/);
  });

  it('на мобилке обёртка прижимает контент к левому краю (к заголовку), а не к правому', () => {
    const bodies = ruleAll(DETAIL_SFC, '.detail-header-actions');
    // Первое объявление - десктопное (flex-end, упирается в крестик), второе -
    // мобильный @media-оверрайд.
    expect(bodies.length).toBeGreaterThanOrEqual(2);
    expect(bodies[0]).toMatch(/justify-content:\s*flex-end/);
    expect(bodies[1]).toMatch(/justify-content:\s*flex-start/);
  });
});

// "Дополнение №4 Согласовать Отказать" на телефоне: метка+первая кнопка на одной
// строке, вторая кнопка отдельно - блок раздувался (94px). Бейдж раунда теперь
// занимает свою строку целиком (flex-basis:100% рвёт wrap-ряд сразу после него),
// обе кнопки решения идут вместе следующей строкой.
describe('ApplicationActionBar - бейдж раунда дополнения на своей строке на мобилке (#1097 w8)', () => {
  it('.supplement-round-badge получает flex-basis: 100% внутри мобильного @media', () => {
    const bodies = ruleAll(ACTION_BAR_SFC, '.supplement-round-badge');
    const mobileBody = bodies.find((b) => /flex-basis/.test(b));
    expect(mobileBody).toBeTruthy();
    expect(mobileBody).toMatch(/flex-basis:\s*100%/);
  });
});
