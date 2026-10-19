import { describe, it, expect } from 'vitest';
import { documentHistoryChangeText, historyActionText, historyFieldLabel, historyValueText, EMPLOYEE_HISTORY_ACTIONS } from '../passageHistoryActions';

describe('document change presentation', () => {
  it.each([
    ['passport_series_number', 'Изменены паспортные данные', 'Паспортные данные'],
    ['patent_number', 'Изменён номер патента', 'Номер патента'],
    ['other_permission', 'Изменено иное разрешение на работы', 'Иное разрешение на работы'],
  ])('%s shows the fact, including legacy values', (field, text, label) => {
    for (const action of ['data_changed', 'update']) {
      const item = { action_type: action, field_name: field, old_value: 'SYNTHETIC-OLD-DOC', new_value: 'SYNTHETIC-NEW-DOC' };
      expect(documentHistoryChangeText(item)).toBe(text);
      expect(historyActionText(item, EMPLOYEE_HISTORY_ACTIONS)).toBe(text);
      expect(historyFieldLabel(field)).toBe(label);
      expect(historyValueText(item, item.old_value)).toBe('');
      expect(historyValueText(item, item.new_value)).toBe('');
      expect(item.old_value).toBe('SYNTHETIC-OLD-DOC');
    }
  });

  it('keeps ordinary fields and period presentation', () => {
    expect(documentHistoryChangeText({ field_name: 'position' })).toBe('');
    expect(historyValueText({ field_name: 'position' }, 'Fixture role')).toBe('Fixture role');
    expect(historyValueText({ field_name: 'period' }, 'inherit: fixture')).toBe('Срок вложения: fixture');
    expect(historyActionText({ action_type: 'entry' }, EMPLOYEE_HISTORY_ACTIONS)).toBe('Проход на территорию');
    expect(documentHistoryChangeText({ field_name: 'constructor' })).toBe('');
    expect(historyFieldLabel('constructor')).toBe('constructor');
  });
});
