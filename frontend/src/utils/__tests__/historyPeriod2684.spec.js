import { describe, it, expect } from 'vitest';
import { historyActionText, CAR_HISTORY_ACTIONS, EMPLOYEE_HISTORY_ACTIONS, historyValueText, historyFieldLabel } from '../passageHistoryActions';

describe('period history presentation', () => {
  it.each([CAR_HISTORY_ACTIONS, EMPLOYEE_HISTORY_ACTIONS])('labels period changes for both kinds', actions => {
    expect(historyActionText({ action_type: 'dates_changed' }, actions)).toBe('Изменён срок действия');
  });
  it('translates both directions without altering dates or source records', () => {
    const item = { field_name: 'period', old_value: 'inherit: 01.02.2030 09:00 - 02.02.2030 18:00', new_value: 'individual: 01.02.2030 09:00 - 03.02.2030 18:00' };
    const original = { ...item };
    expect(historyValueText(item, item.old_value)).toBe('Срок вложения: 01.02.2030 09:00 - 02.02.2030 18:00');
    expect(historyValueText(item, item.new_value)).toBe('Индивидуальный срок: 01.02.2030 09:00 - 03.02.2030 18:00');
    expect(item).toEqual(original);
    expect(historyFieldLabel('period')).toBe('Срок действия');
  });
  it('preserves unknown and empty historical values and unrelated fields', () => {
    expect(historyValueText({ field_name: 'period' }, 'legacy value')).toBe('legacy value');
    expect(historyValueText({ field_name: 'period' }, null)).toBeNull();
    expect(historyValueText({ field_name: 'comment' }, 'individual: text')).toBe('individual: text');
    expect(historyFieldLabel('comment')).toBe('comment');
  });
});
