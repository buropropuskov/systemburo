import { describe, it, expect } from 'vitest'
import { formatShortName, formatFullName, formatUserLabel, formatLogin, formatPublicUserName } from '../formatName'

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

  it('не подставляет логин вместо отсутствующего имени', () => {
    expect(formatUserLabel({ last_name: null, first_name: null, middle_name: null, username: 'ivanov' }))
      .toBe('Пользователь');
  });

  it('пустого пользователя не превращает в мусор', () => {
    expect(formatUserLabel(null)).toBe('');
    expect(formatUserLabel({})).toBe('Пользователь');
  });

  it('показывает серверную маску отдельно от редактируемого ФИО', () => {
    const user = { username: 'fixture', last_name: null, display_label: ' Оператор бюро ' }
    expect(formatUserLabel(user)).toBe('Оператор бюро')
    expect(user.last_name).toBeNull()
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

 describe('public identity privacy', () => {
  for (const format of [formatUserLabel, formatPublicUserName]) {
    it(format.name + ': restriction wins over stale names and alias', () => {
      const user = { pd_hidden: true, display_label: 'Оператор', last_name: 'Скрытое', username: 'private-login' }
      expect(format(user)).toBe('Имя скрыто')
      expect(user.display_label).toBe('Оператор')
    })
    it(format.name + ': explicit alias is preserved; raw login is not', () => {
      expect(format({ display_label: '@Alias', username: 'private-login' })).toBe('@Alias')
      expect(format({ username: 'private-login' })).toBe('Пользователь')
    })
  }
})
