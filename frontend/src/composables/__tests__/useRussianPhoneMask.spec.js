import { describe, it, expect } from 'vitest';
import {
  formatRussianPhone,
  phoneToE164,
  isValidRussianPhone,
  caretAfterMask,
  dropAdjacentDigit,
  applyPhoneMask,
} from '../useRussianPhoneMask';

describe('formatRussianPhone', () => {
  it.each([
    ['9', '+7 (9'],
    ['91', '+7 (91'],
    ['916', '+7 (916)'],
    ['9161', '+7 (916) 1'],
    ['916123', '+7 (916) 123'],
    ['9161234', '+7 (916) 123 4'],
    ['916123456', '+7 (916) 123 45-6'],
    ['9161234567', '+7 (916) 123 45-67'],
  ])('маскирует частичный ввод "%s" в "%s"', (input, expected) => {
    expect(formatRussianPhone(input)).toBe(expected);
  });

  it.each([
    ['89161234567', '+7 (916) 123 45-67'],
    ['79161234567', '+7 (916) 123 45-67'],
    ['+7 (916) 123-45-67', '+7 (916) 123 45-67'],
    ['abc+7(916)def123-45-67', '+7 (916) 123 45-67'],
  ])('приводит "%s" к единому виду', (input, expected) => {
    expect(formatRussianPhone(input)).toBe(expected);
  });

  it('обрезает лишние цифры сверх 11', () => {
    expect(formatRussianPhone('791612345678888')).toBe('+7 (916) 123 45-67');
  });

  it.each([['', ''], [null, ''], [undefined, ''], ['абв', '']])(
    'возвращает пустую строку для "%s"',
    (input, expected) => {
      expect(formatRussianPhone(input)).toBe(expected);
    }
  );
});

describe('isValidRussianPhone', () => {
  it.each(['+7 (916) 123 45-67', '79161234567', '89161234567', '+7 (495) 123 45-67', '+7 (342) 123 45-67'])(
    'принимает корректный номер "%s"',
    (input) => {
      expect(isValidRussianPhone(input)).toBe(true);
    }
  );

  it.each([
    ['+7 (916) 123 45-6', 'недобранная цифра'],
    ['9161234567', '10 цифр без кода страны'],
    ['19161234567', 'чужой код страны'],
    ['71161234567', 'код региона начинается с 1'],
    ['70161234567', 'код региона начинается с 0'],
    ['', 'пусто'],
    [null, 'null'],
  ])('отклоняет "%s" (%s)', (input) => {
    expect(isValidRussianPhone(input)).toBe(false);
  });
});

describe('caretAfterMask', () => {
  it('держит каретку в конце при наборе с конца', () => {
    const masked = formatRussianPhone('916');
    expect(caretAfterMask('916', 3, masked)).toBe(masked.length);
  });

  it('перепрыгивает закрывающую скобку после третьей цифры', () => {
    // "+7 (916)" - каретка должна встать за ")", а не перед ним
    expect(caretAfterMask('916', 3, '+7 (916)')).toBe(8);
  });

  it('держит каретку у той же цифры при правке середины', () => {
    // "+7 (916) 123 45-67", каретка после первой "1" в "123"
    const value = '+7 (916) 123 45-67';
    expect(caretAfterMask(value, 10, value)).toBe(10);
  });

  it('учитывает дописанный маской код страны', () => {
    // Пользователь набрал "9", маска дала "+7 (9" - каретка за девяткой
    expect(caretAfterMask('9', 1, '+7 (9')).toBe(5);
  });

  it('ставит каретку в начало пустого значения', () => {
    expect(caretAfterMask('', 0, '')).toBe(0);
  });
});

describe('dropAdjacentDigit', () => {
  it('убирает цифру слева, когда стёрли закрывающую скобку', () => {
    // "+7 (916) 123 45-67" -> Backspace съел ")", каретка на его месте
    expect(dropAdjacentDigit('+7 (916 123 45-67', 7)).toEqual({
      value: '+7 (91 123 45-67',
      caret: 6,
    });
  });

  it('убирает цифру справа при Delete', () => {
    expect(dropAdjacentDigit('+7 (916 123 45-67', 7, true)).toEqual({
      value: '+7 (916 23 45-67',
      caret: 8,
    });
  });

  it('оставляет значение как есть, если цифры рядом нет', () => {
    expect(dropAdjacentDigit('+', 1)).toEqual({ value: '+', caret: 1 });
  });
});

describe('applyPhoneMask', () => {
  const fakeInput = (value, caret) => ({
    value,
    selectionStart: caret,
    setSelectionRange(from) {
      this.selectionStart = from;
    },
  });

  it('пишет маску в поле и возвращает её же для модели', () => {
    const input = fakeInput('916', 3);
    expect(applyPhoneMask(input, { inputType: 'insertText' }, '')).toBe('+7 (916)');
    expect(input.value).toBe('+7 (916)');
    expect(input.selectionStart).toBe(8);
  });

  it('стирание разделителя убирает цифру, а не оставляет поле как было', () => {
    const input = fakeInput('+7 (916 123 45-67', 7);
    const masked = applyPhoneMask(input, { inputType: 'deleteContentBackward' }, '+7 (916) 123 45-67');
    expect(masked).toBe('+7 (911) 234 56-7');
  });

  it('обычное стирание цифры разделитель не трогает', () => {
    const input = fakeInput('+7 (916) 123 45-6', 17);
    const masked = applyPhoneMask(input, { inputType: 'deleteContentBackward' }, '+7 (916) 123 45-67');
    expect(masked).toBe('+7 (916) 123 45-6');
  });
});

describe('phoneToE164', () => {
  it('снимает маску до +7XXXXXXXXXX', () => {
    expect(phoneToE164('+7 (916) 123 45-67')).toBe('+79161234567');
  });

  it('приводит ведущую 8 к 7', () => {
    expect(phoneToE164('89161234567')).toBe('+79161234567');
  });

  it('возвращает null для пустого номера', () => {
    expect(phoneToE164('')).toBeNull();
    expect(phoneToE164(null)).toBeNull();
    expect(phoneToE164('---')).toBeNull();
  });
});
