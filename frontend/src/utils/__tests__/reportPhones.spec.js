import { describe, it, expect } from 'vitest';
import { formatPhonesInText } from '../reportColumns';

describe('formatPhonesInText (#2336)', () => {
  it('номер ответственного приводится к общей маске сайта', () => {
    expect(formatPhonesInText('Системный администратор, 89100530055'))
      .toBe('Системный администратор, +7 (910) 053 00-55');
  });

  it('номер, уже записанный с разделителями, не задваивает маску', () => {
    expect(formatPhonesInText('Иванов Иван, +7 910 053-00-55'))
      .toBe('Иванов Иван, +7 (910) 053 00-55');
  });

  it('период и время работ не принимаются за номер', () => {
    expect(formatPhonesInText('15.08.2026 - 31.08.2026')).toBe('15.08.2026 - 31.08.2026');
    expect(formatPhonesInText('08:00 - 18:00')).toBe('08:00 - 18:00');
    expect(formatPhonesInText('№ 20260815/001')).toBe('№ 20260815/001');
  });

  it('короткий или нероссийский номер остаётся как есть', () => {
    expect(formatPhonesInText('Охрана, 112')).toBe('Охрана, 112');
    expect(formatPhonesInText('Контакт, 375291234567')).toBe('Контакт, 375291234567');
  });

  it('пустое значение не ломает', () => {
    expect(formatPhonesInText(null)).toBe('');
    expect(formatPhonesInText('')).toBe('');
  });
});
