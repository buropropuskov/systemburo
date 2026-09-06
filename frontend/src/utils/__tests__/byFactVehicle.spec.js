import { describe, it, expect } from 'vitest';
import { isByFactVehicle, hasByFactVehicle, isOneDayPeriod, byFactDeadline, byFactPeriodBroken } from '../byFactVehicle';

/**
 * Форма зеркалит правило бэкенда про машину «По факту» (#2320), чтобы человек
 * узнавал об ограничении в момент действия, а не тостом после заполнения всей
 * формы - именно так на него и наткнулись.
 */
describe('isByFactVehicle', () => {
  it('узнаёт «По факту» в любом написании', () => {
    expect(isByFactVehicle({ car_number: 'По факту' })).toBe(true);
    expect(isByFactVehicle({ car_number: 'по факту' })).toBe(true);
    expect(isByFactVehicle({ car_number: ' ПО ФАКТУ ' })).toBe(true);
    expect(isByFactVehicle({ number: 'По факту' })).toBe(true);
  });

  it('видит номер в поле формы подачи - оно зовётся plateNumber', () => {
    // Форма держит машину как { plateNumber, mark, ... } - проверка только по
    // car_number была зелёной в тестах и молчала в браузере: вторая «По факту»
    // спокойно добавлялась в список.
    expect(isByFactVehicle({ plateNumber: 'По факту', mark: 'По факту' })).toBe(true);
    expect(isByFactVehicle({ plateNumber: 'A123AA777' })).toBe(false);
  });

  it('обычный номер не путает с «По факту»', () => {
    expect(isByFactVehicle({ car_number: 'A123AA777' })).toBe(false);
    expect(isByFactVehicle({})).toBe(false);
    expect(isByFactVehicle(null)).toBe(false);
  });
});

describe('hasByFactVehicle', () => {
  const обычная = { car_number: 'A123AA777' };
  const поФакту = { car_number: 'По факту' };
  const изФормы = { plateNumber: 'По факту' };

  it('видит машину в другом вложении - правило про заявку целиком', () => {
    // Считать по одному вложению мало: во втором бланке форма пропустила бы
    // вторую такую машину, а сервер отклонил бы заявку.
    expect(hasByFactVehicle({ 'бланк-1': [обычная], 'бланк-2': [поФакту] })).toBe(true);
  });

  it('принимает и плоский список - так его зовёт форма транспорта', () => {
    expect(hasByFactVehicle([обычная, поФакту])).toBe(true);
    expect(hasByFactVehicle([обычная])).toBe(false);
  });

  it('считает и машины, добавленные через форму подачи', () => {
    expect(hasByFactVehicle([обычная, изФормы])).toBe(true);
  });

  it('без «По факту» отвечает нет', () => {
    expect(hasByFactVehicle({ 'бланк-1': [обычная, обычная] })).toBe(false);
    expect(hasByFactVehicle({})).toBe(false);
    expect(hasByFactVehicle(null)).toBe(false);
  });

  it('редактируемая машина себя не блокирует', () => {
    // Иначе правку уже добавленной «По факту» форма запретила бы ей же самой.
    expect(hasByFactVehicle({ 'бланк-1': [поФакту] }, поФакту)).toBe(false);
  });

  it('но вторая такая же блокирует и при редактировании первой', () => {
    const другая = { car_number: 'по факту' };
    expect(hasByFactVehicle({ 'бланк-1': [поФакту, другая] }, поФакту)).toBe(true);
  });
});

describe('крайний срок «По факту»', () => {
  // Конец суток, в которые попадает «сейчас плюс 24 часа»: в 17:38 пятого числа
  // это шестое. Округление вверх и есть запас, без которого предел сползал бы
  // каждую минуту - оформленная в 17:31 заявка через минуту стала бы просроченной.
  const вечер = new Date('2026-09-05T14:38:00Z'); // 17:38 МСК

  it('крайняя дата - завтрашний день по Москве', () => {
    expect(byFactDeadline(вечер)).toBe('06.09.2026');
  });

  it('минутой позже предел тот же', () => {
    expect(byFactDeadline(new Date('2026-09-05T14:39:00Z'))).toBe('06.09.2026');
  });

  it('под конец московских суток предел не убегает вперёд', () => {
    // 23:50 МСК: сутки попадают на шестое, значит крайняя дата - шестое.
    expect(byFactDeadline(new Date('2026-09-05T20:50:00Z'))).toBe('06.09.2026');
  });

  it('срок до крайней даты проходит, дальше - нет', () => {
    expect(isOneDayPeriod({ date_from: '2026-09-05', date_to: '2026-09-05' }, вечер)).toBe(true);
    expect(isOneDayPeriod({ date_from: '2026-09-05', date_to: '2026-09-06' }, вечер)).toBe(true);
    expect(isOneDayPeriod({ date_from: '2026-09-05', date_to: '2026-09-07' }, вечер)).toBe(false);
  });

  it('пустой срок недопустим - бэкенд отклоняет его так же', () => {
    expect(isOneDayPeriod({ date_from: '', date_to: '' }, вечер)).toBe(false);
    expect(isOneDayPeriod({ date_from: '2026-09-05' }, вечер)).toBe(false);
    expect(isOneDayPeriod(null, вечер)).toBe(false);
  });
});

describe('byFactPeriodBroken', () => {
  const вечер = new Date('2026-09-05T14:38:00Z');
  const далеко = { date_from: '2026-09-05', date_to: '2026-10-05' };
  const близко = { date_from: '2026-09-05', date_to: '2026-09-06' };

  it('без машины «По факту» и выключенного тумблера правило молчит', () => {
    expect(byFactPeriodBroken(далеко, [{ plateNumber: 'A123AA777' }], false, вечер)).toBe(false);
  });

  it('включённый тумблер поднимает правило ещё до добавления машины', () => {
    expect(byFactPeriodBroken(далеко, [], true, вечер)).toBe(true);
  });

  it('уже добавленная машина поднимает правило и без тумблера', () => {
    expect(byFactPeriodBroken(далеко, [{ plateNumber: 'По факту' }], false, вечер)).toBe(true);
  });

  it('срок в пределах крайней даты правило не нарушает', () => {
    expect(byFactPeriodBroken(близко, [{ plateNumber: 'По факту' }], true, вечер)).toBe(false);
  });
});
