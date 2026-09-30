import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Замок на планшетную шапку «Списка заявок» в кабинете (#2609, третья волна).
 *
 * Замер до правки: 135px на 768 и 151 на 900-992 против 50 на десктопе. Высоту
 * съедала колоночная раскладка мобильного блока компонента, которая действует до
 * 992 включительно, и выпадающий список области, которому она отдавала целую строку.
 *
 * jsdom не считает ни каскад, ни медиазапросы, поэтому геометрию подтверждает замер
 * в браузере, а тест сторожит сами правила: каждое из них уже один раз отсутствовало
 * и давало ровно ту шапку, на которую пожаловался владелец.
 */
const tablet = readFileSync(resolve(process.cwd(), 'src/assets/tablet.css'), 'utf8');
const component = readFileSync(resolve(process.cwd(), 'src/components/UserApplications.vue'), 'utf8');

/**
 * Все блоки `@media` с заданным условием, склеенные. Блоков с одним условием может
 * быть несколько - брать первый попавшийся значит проверять половину контракта.
 */
function mediaBlocks(source, condition) {
  const mark = `@media ${condition} {`;
  const parts = [];
  for (let from = 0; ; ) {
    const start = source.indexOf(mark, from);
    if (start === -1) break;
    let depth = 0;
    let i = start + mark.length - 1;
    for (; i < source.length; i += 1) {
      if (source[i] === '{') depth += 1;
      else if (source[i] === '}') {
        depth -= 1;
        if (depth === 0) break;
      }
    }
    parts.push(source.slice(start + mark.length, i));
    from = i;
  }
  return parts.join('\n');
}

/** Правило по селектору внутри уже вырезанного куска. */
function rule(block, selector) {
  const at = block.indexOf(`${selector} {`);
  if (at === -1) return '';
  return block.slice(at, block.indexOf('}', at));
}

describe('tablet.css - шапка списка заявок в кабинете', () => {
  const portrait = mediaBlocks(tablet, '(min-width: 768px) and (max-width: 992px)');
  const landscape = mediaBlocks(tablet, '(min-width: 992.02px) and (max-width: 1200px)');

  it('в портрете шапка собрана строкой, а не колонкой мобильного блока', () => {
    expect(portrait, 'нет блока 768-992 в tablet.css').not.toBe('');
    expect(rule(portrait, '.applications-card .card-header')).toContain('flex-direction: row');
  });

  it('выпадающий список области не занимает отдельную строку', () => {
    // Мобильный блок компонента даёт ему `order: 3` и `flex: 1 0 100%` - строку в
    // 744px под контрол шириной 140.
    const dropdown = rule(portrait, '.applications-card .cabinet__filter-dropdown');
    expect(dropdown).toContain('order: 0');
    expect(dropdown).toContain('flex: 0 0 auto');
  });

  it('в ландшафте шапка не переносит ряд настроек', () => {
    expect(landscape, 'нет блока 992.02-1200 в tablet.css').not.toBe('');
    expect(rule(landscape, '.applications-card .card-header')).toContain('flex-wrap: nowrap');
  });

  it('ряд заголовка в ландшафте не сжимается ниже своего содержимого', () => {
    // `min-width: 0` давал ряду 401px при содержимом в 425: чип «Обновления» уезжал
    // под поле даты. Наезда не видно по scrollWidth шапки - только сравнением краёв.
    const title = rule(landscape, '.applications-card .card-header__title');
    expect(title).toContain('flex: 0 1 auto');
    expect(title, 'min-width: 0 возвращает наезд чипа на поле даты').not.toContain('min-width: 0');
  });
});

describe('tablet.css - кнопка обновления в ландшафте', () => {
  const landscape = mediaBlocks(tablet, '(min-width: 992.02px) and (max-width: 1200px)');

  it('обновление иконкой, иначе шапка не встаёт одной строкой', () => {
    // Подпись «Обновить» стоит 60px: с ней содержимому шапки нужно 976 при 952
    // доступных на 1024. Ниже 992 кружок рисует мобильный блок компонента.
    expect(rule(landscape, '.applications-card .card-header__settings .refresh-btn')).toContain('border-radius: 50%');
    expect(rule(landscape, '.applications-card .card-header__settings .refresh-btn__text')).toContain('display: none');
  });

  it('правило лежит в глобальном слое, а не в переполненном компоненте', () => {
    // `UserApplications.vue` выше порога храповика build/check-file-sizes.js:
    // добавленная в его <style> строка красит CI-гейт lint:size.
    expect(component).not.toContain('иконка-кружок без подписи');
  });
});
