import { describe, it, expect } from 'vitest';
import { isByFactVehicle, hasByFactVehicle } from '../byFactVehicle';

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

  it('обычный номер не путает с «По факту»', () => {
    expect(isByFactVehicle({ car_number: 'A123AA777' })).toBe(false);
    expect(isByFactVehicle({})).toBe(false);
    expect(isByFactVehicle(null)).toBe(false);
  });
});

describe('hasByFactVehicle', () => {
  const обычная = { car_number: 'A123AA777' };
  const поФакту = { car_number: 'По факту' };

  it('видит машину в другом вложении - правило про заявку целиком', () => {
    // Считать по одному вложению мало: во втором бланке форма пропустила бы
    // вторую такую машину, а сервер отклонил бы заявку.
    expect(hasByFactVehicle({ 'бланк-1': [обычная], 'бланк-2': [поФакту] })).toBe(true);
  });

  it('принимает и плоский список - так его зовёт форма транспорта', () => {
    expect(hasByFactVehicle([обычная, поФакту])).toBe(true);
    expect(hasByFactVehicle([обычная])).toBe(false);
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
