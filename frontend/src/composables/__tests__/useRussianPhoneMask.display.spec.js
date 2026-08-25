import { describe, it, expect } from 'vitest';
import { formatRussianPhoneForDisplay } from '../useRussianPhoneMask';

describe('formatRussianPhoneForDisplay', () => {
  it('приводит к маске номер, сохранённый цифрами подряд', () => {
    expect(formatRussianPhoneForDisplay('79100830055')).toBe('+7 (910) 083 00-55');
    expect(formatRussianPhoneForDisplay('89100830055')).toBe('+7 (910) 083 00-55');
    expect(formatRussianPhoneForDisplay('9100830055')).toBe('+7 (910) 083 00-55');
  });

  it('уже отформатированный номер не портит', () => {
    expect(formatRussianPhoneForDisplay('+7 (910) 083 00-55')).toBe('+7 (910) 083 00-55');
  });

  it('оставляет как есть всё, что не похоже на российский номер', () => {
    // Маска выкинула бы добавочный и обрезала бы второй номер.
    expect(formatRussianPhoneForDisplay('+7 495 123-45-67 доб. 12')).toBe('+7 495 123-45-67 доб. 12');
    expect(formatRussianPhoneForDisplay('112')).toBe('112');
    expect(formatRussianPhoneForDisplay('+1 202 555 0123')).toBe('+1 202 555 0123');
  });

  it('пустое значение остаётся пустым', () => {
    expect(formatRussianPhoneForDisplay('')).toBe('');
    expect(formatRussianPhoneForDisplay(null)).toBe('');
    expect(formatRussianPhoneForDisplay(undefined)).toBe('');
  });
});
