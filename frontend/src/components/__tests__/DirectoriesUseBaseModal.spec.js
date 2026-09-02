import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Справочники не заводят своих модалок (#871).
 *
 * Восемь админских справочников написаны копипастой, и каждая правка оформления
 * размазывается по всем восьми. Модалка - самый дорогой её кусок: свой
 * `Teleport`, свой оверлей, свои обработчики закрытия по фону, свои 70-85 строк
 * стилей на файл. У `BaseModal` всё это уже есть, включая закрытие по Escape со
 * стопкой открытых окон и swipe-лист на мобилке.
 *
 * Замок текстовый: поднимать восемь компонентов с их сторами ради одной проверки
 * дороже, чем сверить разметку. Он же держит и второй конец - собственные стили
 * модалки, которые иначе остаются сиротами после перевода.
 */

const componentsDir = resolve(__dirname, '..');

const DIRECTORIES = [
  'OrganizationsManagement',
  'CompaniesManagement',
  'CitizenshipManagement',
  'MarksManagement',
  'DocumentsManagement',
  'AttachmentsManagement',
  'NewsManagement',
  'GuideManagement',
];

const source = (name) => readFileSync(resolve(componentsDir, `${name}.vue`), 'utf8');

/** Разметка компонента - до открытия script. */
const markup = (src) => src.slice(0, src.indexOf('<script'));

describe('справочники: модалки только через BaseModal', () => {
  it.each(DIRECTORIES)('%s: нет своего Teleport под модалку', (name) => {
    expect(
      /<Teleport|<teleport/.test(markup(source(name))),
      'своя модалка через Teleport: у неё нет ни закрытия по Escape со стопкой окон, '
        + 'ни swipe-листа на мобилке, а её оформление придётся править отдельно от '
        + 'остальных семи справочников',
    ).toBe(false);
  });

  it.each(DIRECTORIES)('%s: нет своего оверлея и его стилей', (name) => {
    const src = source(name);
    expect(
      /class="modal-overlay"/.test(markup(src)),
      'свой оверлей вместо BaseModal - закрытие по фону придётся писать руками',
    ).toBe(false);
    expect(
      /^\.modal-(overlay|content|header|body|footer|close|fade)/m.test(src),
      'остались стили собственной модалки: после перевода они сироты и живут, '
        + 'пока кто-нибудь не заметит',
    ).toBe(false);
  });

  it('перевод не оставил ручного закрытия по фону', () => {
    // useOverlayClose нужен тому, кто рисует оверлей сам. У BaseModal это внутри.
    const offenders = DIRECTORIES.filter((name) => {
      const src = source(name);
      return src.includes('useOverlayClose') && !markup(src).includes('modal-overlay');
    });

    expect(
      offenders,
      'композабл закрытия по фону подключён, но своего оверлея в разметке нет - '
        + 'мёртвая оснастка после перевода на BaseModal',
    ).toEqual([]);
  });
});
