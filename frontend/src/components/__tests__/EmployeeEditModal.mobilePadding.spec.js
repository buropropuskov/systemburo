import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// Волна 7 перевела модалку добавления сотрудника на BaseModal, а вместе со старой
// разметкой ушли и отступы содержимого - "отступов нет в модалке добавления" (разбор
// второго круга замечаний владельца, #1097 w8). BaseModal.base-modal__body идёт БЕЗ
// padding намеренно (см. BaseModal.vue), поэтому отступ обязан задавать сам потребитель.
const SFC = readFileSync(resolve(__dirname, '../EmployeeEditModal.vue'), 'utf8');

/** Тело правила для селектора, без учёта переносов (первое совпадение в источнике). */
function rule(src, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const found = src.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`));
  return found ? found[1].replace(/\s+/g, ' ').trim() : null;
}

/** Содержимое @media-блока по маркеру начала (со сбалансированным подсчётом скобок -
 *  без этого rule() нашёл бы базовое (не мобильное) правило .binding-option). */
function mediaBlock(src, marker) {
  const start = src.indexOf(marker);
  if (start === -1) return null;
  let i = src.indexOf('{', start) + 1;
  let depth = 1;
  const bodyStart = i;
  while (depth > 0 && i < src.length) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') depth--;
    i++;
  }
  return src.slice(bodyStart, i - 1);
}

const MOBILE_768 = mediaBlock(SFC, '@media (max-width: 768px)');

describe('EmployeeEditModal — отступы формы и подписи привязки на мобилке', () => {
  it('data__completion несёт реальный padding, а не 0', () => {
    expect(rule(SFC, '.data__completion')).toMatch(/padding:\s*14px 20px 18px/);
  });

  it('на телефоне подписи чекбоксов привязки крупнее и тач-таргет строки не мельче 36px', () => {
    const bindingRule = rule(MOBILE_768, '.binding-option');
    expect(bindingRule, 'мобильный оверрайд .binding-option не найден').not.toBeNull();
    expect(bindingRule).toMatch(/font-size:\s*14px/);
    expect(bindingRule).toMatch(/min-height:\s*36px/);

    const checkboxRule = rule(MOBILE_768, '.binding-option input[type="checkbox"]');
    expect(checkboxRule, 'мобильный оверрайд чекбокса привязки не найден').not.toBeNull();
    expect(checkboxRule).toMatch(/width:\s*18px/);
    expect(checkboxRule).toMatch(/height:\s*18px/);
  });
});
