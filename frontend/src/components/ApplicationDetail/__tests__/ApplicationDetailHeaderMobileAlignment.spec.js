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

// Волна 11 (владелец): бейдж "Доп. №N" в самом ряду решения дублировал заголовок
// "+ Дополнение №N на согласовании" из шапки заявки, а на мобилке (flex-basis:100%)
// растягивался на всю ширину пилюлей без текста. Бейдж убран целиком - номер несёт
// подпись кнопки ("Согласовать доп. №N"), а сам ряд решения принудительно встаёт под
// заголовком шапки (.action-bar-root получает flex-basis:100% в .detail-header-actions),
// а не бейдж внутри себя.
describe('ApplicationActionBar - бейдж "Доп. №N" в ряду решения убран (#1097 w11)', () => {
  it('в разметке ActionBar нет бейджа с testid supplement-round-badge', () => {
    expect(ACTION_BAR_SFC).not.toMatch(/data-testid="supplement-round-badge"/);
  });

  it('ряд решения по-прежнему переносится (flex-wrap: wrap) - кнопки не сжаты в одну строку', () => {
    const body = rule(ACTION_BAR_SFC, '.supplement-actions');
    expect(body).toMatch(/flex-wrap:\s*wrap/);
  });

  // Ряд решения обведён собственной рамкой/подложкой не был - убран волной w12
  // (владелец: "убрать блок, обводку с тёмным фоном"). Кнопки решения стоят
  // прямо в шапке, без визуального контейнера вокруг ряда.
  it('у ряда решения нет рамки и подложки (нет визуального контейнера)', () => {
    const body = rule(ACTION_BAR_SFC, '.supplement-actions');
    expect(body).not.toMatch(/border(-\w+)?:\s*1px\s+solid/);
    expect(body).not.toMatch(/background:/);
    expect(body).not.toMatch(/padding:\s*6px/);
  });

  // Волна w12 (владелец): "Дополнение" + "Отозвать" + "В работе" на десктопе
  // разъезжались на две строки - панель действий выталкивалась под бейдж
  // ЛЮБОЙ шириной экрана. flex-basis:100% остаётся верным только для мобилки,
  // где под шапку и правда нужно уйти отдельной строкой; на десктопе всё
  // должно стоять в одну строку с бейджем.
  it('панель действий заявки встаёт под бейджем "+ Дополнение..." ТОЛЬКО на мобилке (flex-basis: 100%)', () => {
    const mobileMarker = DETAIL_SFC.indexOf('.detail-header-actions {\n        justify-content: flex-start;');
    expect(mobileMarker).toBeGreaterThan(-1);

    const actionBarRuleIndex = DETAIL_SFC.indexOf('.detail-header-actions :deep(.action-bar-root)');
    expect(actionBarRuleIndex).toBeGreaterThan(mobileMarker);

    const body = rule(DETAIL_SFC, '.detail-header-actions :deep(.action-bar-root)');
    expect(body).toMatch(/flex-basis:\s*100%/);

    // Вне мобильного блока правило не объявлено - на десктопе панель не
    // выталкивается под бейдж отдельной строкой.
    const beforeMobile = DETAIL_SFC.slice(0, mobileMarker);
    expect(beforeMobile).not.toMatch(/\.detail-header-actions :deep\(\.action-bar-root\)/);
  });
});
