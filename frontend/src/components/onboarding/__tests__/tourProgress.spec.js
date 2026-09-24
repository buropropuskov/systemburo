import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { readProgress, saveProgress, clearProgress } from '../tourProgress';
import { TOURS } from '../tours';

/**
 * Обучение заявителя идёт под шестьдесят шагов. Раньше любой перерыв стоил всего
 * пройденного: перезагрузил страницу на сороковом шаге - и тур начинался заново.
 * Замки стерегут, что позиция переживает перерыв, не путается между людьми за
 * одним компьютером и не воскресает через месяц.
 */
describe('позиция в туре', () => {
  beforeEach(() => localStorage.clear());

  it('сохраняется и читается', () => {
    saveProgress(7, 'user', 23);
    expect(readProgress(7, 'user').index).toBe(23);
  });

  it('у каждого человека своя - за одним компьютером работают посменно', () => {
    saveProgress(7, 'user', 23);
    expect(readProgress(8, 'user').index).toBe(0);
  });

  it('у каждого тура своя', () => {
    saveProgress(7, 'user', 23);
    expect(readProgress(7, 'guard').index).toBe(0);
  });

  it('первый шаг не запоминаем - продолжать с него нечего', () => {
    saveProgress(7, 'user', 0);
    expect(readProgress(7, 'user').index).toBe(0);
  });

  it('досмотренный тур позицию стирает', () => {
    saveProgress(7, 'user', 23);
    clearProgress(7, 'user');
    expect(readProgress(7, 'user').index).toBe(0);
  });

  it('через две недели запись протухает - обучение честнее начать сначала', () => {
    const old = Date.now() - 15 * 24 * 60 * 60 * 1000;
    localStorage.setItem('ob:progress:7:user', JSON.stringify({ index: 23, at: old }));
    expect(readProgress(7, 'user').index).toBe(0);
    expect(localStorage.getItem('ob:progress:7:user')).toBe(null);
  });

  it('тур, оставшийся на экране, помечен прерванным - его поднимут сами', () => {
    saveProgress(7, 'user', 23, true);
    expect(readProgress(7, 'user').interrupted).toBe(true);
  });

  it('закрытый человеком тур прерванным не считается - ждёт меню', () => {
    saveProgress(7, 'user', 23, false);
    expect(readProgress(7, 'user').interrupted).toBe(false);
  });

  it('битая запись читается как «начать сначала», а не роняет тур', () => {
    localStorage.setItem('ob:progress:7:user', '{это не json');
    expect(readProgress(7, 'user').index).toBe(0);
  });

  it('позиция из прошлой версии тура не читается', () => {
    // Тур переписывают: шаги меняются местами, и старый индекс указывает не туда.
    // Хуже того, если шаг жил на другой странице, продолжение с него гасило тур
    // сразу после запуска - сегмента для чужого пути нет (#2584).
    const версия = TOURS.find((t) => t.key === 'accept').version;
    localStorage.setItem(
      'ob:progress:7:accept',
      JSON.stringify({ index: 11, at: Date.now(), onScreen: false, version: версия - 1 }),
    );
    expect(readProgress(7, 'accept').index).toBe(0);
    expect(localStorage.getItem('ob:progress:7:accept'), 'протухшая запись убрана').toBe(null);
  });

  it('позиция текущей версии продолжается как раньше', () => {
    saveProgress(7, 'accept', 5);
    expect(readProgress(7, 'accept').index).toBe(5);
  });

  describe('хранилище недоступно (приватный режим)', () => {
    let spy;
    afterEach(() => spy?.mockRestore());

    it('запись молча пропускается', () => {
      spy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new Error('QuotaExceeded');
      });
      expect(() => saveProgress(7, 'user', 23)).not.toThrow();
    });

    it('чтение отдаёт начало тура', () => {
      spy = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
        throw new Error('SecurityError');
      });
      expect(readProgress(7, 'user').index).toBe(0);
    });
  });
});
