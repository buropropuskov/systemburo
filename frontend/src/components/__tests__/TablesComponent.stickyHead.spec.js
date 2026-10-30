import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

const SFC = readFileSync(resolve(__dirname, '../TablesComponent.vue'), 'utf8');

/** Тело мобильного правила для селектора, без учёта переносов. */
function mobileRule(selector, source = SFC) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const found = source.match(new RegExp(`^[ \t]{4}${escaped}\\s*\\{([^}]*)\\}`, 'm'));
  return found ? found[1].replace(/\s+/g, ' ').trim() : null;
}

/**
 * Шапка страницы таблиц поста прилипает СРАЗУ, без проезда.
 *
 * Владелец: «при скролле шапка страницы скроллится на 5px вверх, а потом прилипает».
 * Причина - отступ страницы (`--gutter`, 10-12px на телефоне) стоял СНАРУЖИ липкого
 * блока: пока блок проезжал эту щель до шапки приложения, он ехал вместе со страницей.
 * Лечится переносом отступа внутрь блока - тем же приёмом, что в шапке Центра заявок.
 *
 * jsdom каскад и медиазапросы не считает, поэтому стережём само правило.
 */
describe('TablesComponent — закреплённая шапка страницы', () => {
  const rule = () => mobileRule('.tables__sticky-head');

  it('шапка липкая и встаёт под шапку приложения', () => {
    const decls = rule();

    expect(decls, 'мобильное правило .tables__sticky-head не найдено').not.toBeNull();
    expect(decls).toMatch(/position:\s*sticky/);
    expect(decls).toMatch(/top:\s*var\(--mobile-header-height/);
  });

  it('отступ страницы уведён внутрь блока - щели над ним не остаётся', () => {
    const decls = rule();

    expect(decls, `отрицательный внешний отступ не задан: ${decls}`)
      .toMatch(/margin:\s*calc\(var\(--gutter\) \* -1\)/);
    expect(decls, `внутренний отступ не задан: ${decls}`)
      .toMatch(/padding:\s*var\(--gutter\)/);
  });

  it('фон непрозрачный - содержимое не просвечивает сквозь шапку', () => {
    expect(rule()).toMatch(/background:\s*var\(--/);
  });

  // Линия принадлежит липкому блоку, растянутому до краёв экрана. На поле фильтров
  // она обрывалась по отступу страницы и не доходила до краёв, как в шапке Центра.
  it('линия идёт по самой шапке, а не по полю фильтров', () => {
    expect(rule(), `у шапки нет нижней линии: ${rule()}`).toMatch(/border-bottom:\s*1px solid/);
    expect(mobileRule('.tables__filters'), 'линия осталась и на фильтрах - будет двойная')
      .toMatch(/border-bottom:\s*none/);
  });
});

// Переносы Windows не должны превращать пустые строки перед desktop-правилом
// в отступ мобильного селектора. Проверяем оба формата исходного файла.
it.each([['LF', '\n'], ['CRLF', '\r\n']])('различает отступы и переносы строк (%s)', (_name, newline) => {
  const css = [
    '', '', '.probe { border-bottom: 1px solid black; }',
    '@media (max-width: 767.98px) {',
    '    .probe { border-bottom: none; }', '}',
  ].join(newline);
  expect(mobileRule('.probe', css)).toBe('border-bottom: none;');
});
