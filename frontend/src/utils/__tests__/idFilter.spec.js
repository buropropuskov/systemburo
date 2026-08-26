import { describe, it, expect } from 'vitest';
import { idFilterSet } from '@/utils/idFilter';

describe('idFilterSet (#1398)', () => {
  it('пустой выбор - null, а не пустой Set: фильтр выключен, не «ничего не совпало»', () => {
    expect(idFilterSet([])).toBeNull();
    expect(idFilterSet(null)).toBeNull();
    expect(idFilterSet(undefined)).toBeNull();
  });

  it('не массив (скаляр из старого контракта) не роняет и не фильтрует', () => {
    expect(idFilterSet(7)).toBeNull();
    expect(idFilterSet('7')).toBeNull();
  });

  it('ключи строковые - смешанные типы из справочника и из строк таблицы совпадают', () => {
    const set = idFilterSet([1, '2', 3]);
    expect(set.has('1')).toBe(true);
    expect(set.has('2')).toBe(true);
    expect(set.has(String(3))).toBe(true);
    expect(set.size).toBe(3);
  });

  it('id = 0 попадает в набор, а не отбрасывается как falsy', () => {
    const set = idFilterSet([0]);
    expect(set).not.toBeNull();
    expect(set.has('0')).toBe(true);
  });

  it('null/undefined/пустая строка внутри массива отбрасываются', () => {
    expect(idFilterSet([null, undefined, ''])).toBeNull();
    const set = idFilterSet([null, 5, '']);
    expect(set.size).toBe(1);
    expect(set.has('5')).toBe(true);
  });

  it('дубликаты сворачиваются', () => {
    expect(idFilterSet([4, '4', 4]).size).toBe(1);
  });
});
