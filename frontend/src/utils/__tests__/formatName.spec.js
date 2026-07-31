import { describe, it, expect } from 'vitest'
import { formatShortName, formatFullName, formatUserLabel, formatLogin } from '../formatName'

describe('formatShortName', () => {
  it('полное ФИО -> Фамилия И.О.', () => {
    expect(formatShortName({ last_name: 'Иванов', first_name: 'Иван', middle_name: 'Иванович' }))
      .toBe('Иванов И.И.')
  })

  it('фамилия и имя -> Фамилия И.', () => {
    expect(formatShortName({ last_name: 'Иванов', first_name: 'Иван' })).toBe('Иванов И.')
  })

  it('только фамилия -> Фамилия', () => {
    expect(formatShortName({ last_name: 'Иванов' })).toBe('Иванов')
  })

  it('только имя -> Имя (не сокращаем)', () => {
    expect(formatShortName({ first_name: 'Иван' })).toBe('Иван')
  })

  it('имя и отчество без фамилии -> Имя Отчество (полностью)', () => {
    expect(formatShortName({ first_name: 'Иван', middle_name: 'Иванович' }))
      .toBe('Иван Иванович')
  })

  it('пустые поля -> ""', () => {
    expect(formatShortName({})).toBe('')
    expect(formatShortName(null)).toBe('')
    expect(formatShortName(undefined)).toBe('')
  })

  it('игнорирует пробелы по краям', () => {
    expect(formatShortName({ last_name: '  Иванов ', first_name: ' Иван' }))
      .toBe('Иванов И.')
  })

  it('первая буква в верхнем регистре', () => {
    expect(formatShortName({ last_name: 'иванов', first_name: 'иван' }))
      .toBe('иванов И.')
  })
})

describe('formatFullName', () => {
  it('собирает три части в одну строку', () => {
    expect(formatFullName({ last_name: 'Иванов', first_name: 'Иван', middle_name: 'Иванович' }))
      .toBe('Иванов Иван Иванович')
  })

  it('пропускает пустые части', () => {
    expect(formatFullName({ last_name: 'Иванов', middle_name: 'Иванович' }))
      .toBe('Иванов Иванович')
  })

  it('пустой объект -> ""', () => {
    expect(formatFullName({})).toBe('')
  })
})

describe('formatUserLabel', () => {
  it('показывает сокращённое ФИО, когда оно есть', () => {
    expect(formatUserLabel({ last_name: 'Иванов', first_name: 'Иван', middle_name: 'Иванович', username: 'ivanov' }))
      .toBe('Иванов И.И.');
  });

  it('падает на логин с собачкой, когда ФИО скрыто до согласия на обработку данных', () => {
    expect(formatUserLabel({ last_name: null, first_name: null, middle_name: null, username: 'ivanov' }))
      .toBe('@ivanov');
  });

  it('пустого пользователя не превращает в мусор', () => {
    expect(formatUserLabel(null)).toBe('');
    expect(formatUserLabel({})).toBe('');
  });
});

describe('formatLogin', () => {
  it('добавляет собачку', () => {
    expect(formatLogin('ivanov')).toBe('@ivanov');
  });

  it('вторую собачку не добавляет', () => {
    expect(formatLogin('@ivanov')).toBe('@ivanov');
  });

  it('пустой логин оставляет пустым, чтобы не рисовать одинокую собачку', () => {
    expect(formatLogin('')).toBe('');
    expect(formatLogin(null)).toBe('');
  });
});
