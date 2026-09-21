import { describe, it, expect } from 'vitest';
import { passageSourceLabel, passageSourceVariant } from '../passageSource';

describe('подпись источника места прохода', () => {
  it('различает все три пути появления привязки', () => {
    expect(passageSourceLabel('application')).toBe('из заявки');
    expect(passageSourceLabel('approver')).toBe('назначил принимающий');
    expect(passageSourceLabel('manual')).toBe('добавлено вручную');
  });

  it('молчит, когда источник неизвестен', () => {
    expect(passageSourceLabel(null)).toBe('');
    expect(passageSourceLabel(undefined)).toBe('');
    expect(passageSourceLabel('что-то новое')).toBe('');
  });

  it('выделяет только указанное самим заявителем', () => {
    expect(passageSourceVariant('application')).toBe('primary');
    expect(passageSourceVariant('approver')).toBe('neutral');
    expect(passageSourceVariant('manual')).toBe('neutral');
  });
});
