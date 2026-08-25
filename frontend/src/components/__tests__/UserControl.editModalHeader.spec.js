import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

const SFC = readFileSync(resolve(__dirname, '../UserControl.vue'), 'utf8');

/** Разметка шапки окна редактирования: от ряда действий до тела окна. */
function headerMarkup() {
  const start = SFC.indexOf('class="modal-header-actions"');
  const end = SFC.indexOf('class="modal-body-inner"', start);
  return SFC.slice(start, end);
}

/**
 * Шапка окна редактирования пользователя.
 *
 * У охранника действий на одно больше остальных ролей («Места доступа»), и ряд
 * переставал помещаться в окно шириной 880: кнопки вылезали за край. Окно шире,
 * кнопки компактные, ряд умеет переноситься - три условия вместе, снятие любого
 * возвращает дефект.
 */
describe('UserControl — шапка окна редактирования', () => {
  it('окно достаточно широкое для ряда действий охранника', () => {
    const width = SFC.match(/:show="showEditModal && !!selectedUser"\s*\n\s*width="(\d+)px"/);

    expect(width, 'ширина окна редактирования не найдена').not.toBeNull();
    expect(Number(width[1])).toBeGreaterThanOrEqual(1040);
  });

  it('кнопки шапки компактные', () => {
    const markup = headerMarkup();
    const buttons = markup.match(/class="lk-button[^"]*"/g) || [];

    expect(buttons.length, 'кнопок в шапке не найдено').toBeGreaterThan(0);
    for (const cls of buttons) {
      expect(cls, `кнопка шапки не компактная: ${cls}`).toContain('lk-button--sm');
    }
  });

  it('ряд действий переносится, а не вылезает за край', () => {
    const rule = SFC.match(/\.modal-header-actions\s*\{([^}]*)\}/);

    expect(rule, 'правило .modal-header-actions не найдено').not.toBeNull();
    expect(rule[1]).toMatch(/flex-wrap:\s*wrap/);
  });
});
