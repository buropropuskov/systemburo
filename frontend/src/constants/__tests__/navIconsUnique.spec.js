import { describe, it, expect } from 'vitest';
import { ADMIN_GROUPS } from '../navSections';
import { navIcons } from '@/components/icons/navIcons';

/**
 * Значок пункта меню обязан быть своим (#2461).
 *
 * Три пункта секции «Доступ и роли» - «Журнал отказов», «Доступ к перс. данным» и
 * «Сведения о человеке» - несли один и тот же `access-denials` и в меню стояли
 * рядом неразличимой троицей. Ничто не мешало скопировать чужое имя при добавлении
 * раздела, поэтому дубль сторожим тестом, а не внимательностью.
 */
const items = ADMIN_GROUPS.flatMap((g) => g.items.map((i) => ({ ...i, group: g.title })));

describe('иконки навигации', () => {
  it('ни один значок не занят дважды', () => {
    const seen = new Map();
    const duplicates = [];
    for (const item of items) {
      if (seen.has(item.icon)) {
        duplicates.push(`${item.icon}: «${seen.get(item.icon)}» и «${item.label}»`);
      } else {
        seen.set(item.icon, item.label);
      }
    }
    expect(duplicates, `значки повторяются: ${duplicates.join('; ')}`).toEqual([]);
  });

  it('каждый указанный значок есть в наборе', () => {
    const missing = items.filter((i) => !navIcons[i.icon]).map((i) => `${i.label} -> ${i.icon}`);
    expect(missing, `значок не найден в navIcons: ${missing.join(', ')}`).toEqual([]);
  });

  it('оба журнала персональных данных отличаются от журнала отказов', () => {
    const byLabel = Object.fromEntries(items.map((i) => [i.label, i.icon]));
    expect(byLabel['Доступ к перс. данным']).not.toBe(byLabel['Журнал отказов']);
    expect(byLabel['Сведения о человеке']).not.toBe(byLabel['Журнал отказов']);
    expect(byLabel['Сведения о человеке']).not.toBe(byLabel['Доступ к перс. данным']);
  });
});
