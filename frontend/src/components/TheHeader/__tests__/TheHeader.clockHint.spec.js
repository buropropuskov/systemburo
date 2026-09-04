import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Часы в шапке объясняют себя подсказкой (#2298).
 *
 * Время там московское и сверенное с сервером, поэтому оно расходится с часами
 * компьютера у всех, кто не в Москве или у кого часы сбиты. Без подписи такое
 * расхождение читается как ошибка системы, а не как её работа - подсказка снимает
 * вопрос до того, как он возникнет.
 *
 * Замок текстовый: подсказка рисуется псевдоэлементом `::after`, которого нет в
 * DOM, - смонтированный компонент о ней ничего не скажет (проверено на подсказках
 * тегов Центра, см. lessons/verification.md).
 */

const source = readFileSync(resolve(__dirname, '..', 'TheHeader.vue'), 'utf8');

/** Разметка компонента - до открытия script. */
const markup = source.slice(0, source.indexOf('<script'));

/** Атрибуты элемента с часами. */
function clockTag() {
  const start = markup.indexOf('data-testid="header-time"');
  if (start < 0) return '';
  return markup.slice(markup.lastIndexOf('<span', start), markup.indexOf('>', start) + 1);
}

describe('шапка: подсказка у часов', () => {
  it('часы подписаны как московские', () => {
    const tag = clockTag();
    expect(tag, 'элемент с часами не найден').toBeTruthy();

    expect(
      /data-hint="[^"]*осков/.test(tag),
      'у часов нет подсказки про московское время: расхождение с часами компьютера '
        + 'будет выглядеть как ошибка системы',
    ).toBe(true);
  });

  it('подсказка падает вниз, а не вверх', () => {
    expect(
      /hint-anchor--below/.test(clockTag()),
      'подсказка сверху обрезается границей окна - шапка прижата к верху экрана',
    ).toBe(true);
  });

  it('часы остаются по центру своего места', () => {
    // hint-anchor делает элемент inline-flex, и text-align в нём не работает:
    // текст становится анонимным flex-элементом и прижимается влево.
    // Правил `.header__time` в файле два: короткое с порядком в ряду и основное.
    // Нужное узнаём по min-width - поиск по первому совпадению взял бы не то.
    const rules = [...source.matchAll(/\.header__time \{([^}]*)\}/g)].map((m) => m[1]);
    const body = rules.find((r) => r.includes('min-width')) || '';

    expect(
      /justify-content:\s*center/.test(body),
      'без justify-content время съедет влево от своего места в шапке',
    ).toBe(true);
  });
});
