import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { ACTION_DOT_CLASS, ACTION_TEXT } from '../applicationHistoryActions';

/**
 * Ключи словарей ленты - это значения AuditLog.Action с бэка. Разъехаться они могут
 * молча: неизвестный ключ ленту не роняет, а выводит пользователю сырое
 * `bureau_note_created` вместо подписи. Заметит это только тот, кто откроет историю
 * заявки после нужного действия.
 *
 * Замок сверяет обе стороны для заметки бюро - действия, ради которого историю и
 * заводили (владелец: «почему в истории заявки нет создания заметки и её
 * редактирования»).
 */

const auditSource = readFileSync(
  resolve(__dirname, '..', '..', '..', '..', 'internal', 'models', 'audit_log.go'),
  'utf8',
);

/** Значения констант AuditActionBureauNote* из Go. */
function bureauNoteActions() {
  return [...auditSource.matchAll(/AuditActionBureauNote\w*\s*=\s*"([^"]+)"/g)].map((m) => m[1]);
}

describe('словари ленты истории заявки', () => {
  it('действия заметки бюро описаны с обеих сторон', () => {
    const actions = bureauNoteActions();
    expect(actions.length, 'константы заметки не найдены - переименовали на бэке?').toBe(3);

    for (const action of actions) {
      expect(
        ACTION_TEXT[action],
        `${action}: нет подписи - в ленте появится сырой ключ действия`,
      ).toBeTruthy();
      expect(
        ACTION_DOT_CLASS[action],
        `${action}: нет класса точки - запись встанет в ленте без цветовой метки`,
      ).toBeTruthy();
    }
  });

  it('текст заметки в подписи не участвует', () => {
    // Журнал читают мониторинг и выгрузки, поэтому ни старый, ни новый текст туда не
    // пишется. Подпись обязана говорить о факте, а не подставлять содержимое.
    for (const action of bureauNoteActions()) {
      expect(ACTION_TEXT[action]).not.toMatch(/\$\{|\{\{/);
    }
  });

  it('у каждой подписи есть класс точки', () => {
    const missing = Object.keys(ACTION_TEXT).filter((key) => !ACTION_DOT_CLASS[key]);
    expect(missing, 'действие описано подписью, но без цвета - запись выпадет из ряда').toEqual([]);
  });
});
