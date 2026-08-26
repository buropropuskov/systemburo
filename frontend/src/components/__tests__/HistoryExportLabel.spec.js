import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

/*
 * Кнопка экспорта в историях Т/С и сотрудника: круглая иконка только на узком
 * экране, на десктопе подписана «Экспорт» (как в остальных историях системы).
 * Замок читает SFC, а не рендер: jsdom каскад и @media не считает, поэтому
 * «текст снова убрали» юнит-тест на разметке не поймал бы.
 */
const MODALS = {
  'CarHistoryModal.vue': '../CarHistoryModal.vue',
  'EmployeeHistoryModal.vue': '../CreateApplication/EmployeeHistoryModal.vue',
};

function readSfc(relative) {
  return readFileSync(fileURLToPath(new URL(relative, import.meta.url)), 'utf8');
}

function mobileBlock(source) {
  const start = source.indexOf('@media (max-width: 768px)');
  return start === -1 ? '' : source.slice(start);
}

describe('История Т/С и сотрудника - подпись кнопки экспорта', () => {
  for (const [name, path] of Object.entries(MODALS)) {
    describe(name, () => {
      const source = readSfc(path);

      it('в шапке есть подписанная кнопка экспорта', () => {
        expect(source).toContain('class="export-label"');
        expect(source).toMatch(/class="export-label"\s*>\s*Экспорт\s*<\/span>/);
      });

      it('на десктопе кнопка не круглая иконка', () => {
        const base = source.slice(source.indexOf('\n.export-btn {'), source.indexOf('\n.export-btn:hover'));
        expect(base).toContain('border-radius: 20px');
        expect(base).not.toContain('border-radius: 50%');
      });

      it('на узком экране подпись скрыта, кнопка снова круглая', () => {
        const mobile = mobileBlock(source);
        expect(mobile).toMatch(/\.export-label\s*{\s*display:\s*none;/);
        expect(mobile).toMatch(/\.export-btn\s*{[^}]*border-radius:\s*50%/);
      });
    });
  }
});
