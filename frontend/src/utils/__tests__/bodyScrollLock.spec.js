import { describe, it, expect, beforeEach } from 'vitest';
import { setBodyScrollLock, releaseBodyScrollLock, resetBodyScrollLock } from '../bodyScrollLock';

// Окна на странице заявки живут стопкой (карточка Т/С -> место разгрузки -> таблица).
// Пока каждое ставило overflow само, размонтирование соседа снимало блокировку под
// всё ещё открытым окном (#1097 S4).

describe('замок прокрутки фона', () => {
  const first = {};
  const second = {};

  beforeEach(() => {
    resetBodyScrollLock();
  });

  it('первый владелец блокирует, он же и снимает', () => {
    setBodyScrollLock(first, true);
    expect(document.body.style.overflow).toBe('hidden');
    expect(document.documentElement.style.overflow).toBe('hidden');
    setBodyScrollLock(first, false);
    expect(document.body.style.overflow).toBe('');
    expect(document.documentElement.style.overflow).toBe('');
  });

  it('пока хоть одно окно открыто, закрытие соседнего не отпускает фон', () => {
    setBodyScrollLock(first, true);
    setBodyScrollLock(second, true);
    releaseBodyScrollLock(second);
    expect(document.body.style.overflow).toBe('hidden');
    expect(document.documentElement.style.overflow).toBe('hidden');
    releaseBodyScrollLock(first);
    expect(document.body.style.overflow).toBe('');
    expect(document.documentElement.style.overflow).toBe('');
  });

  it('повторная блокировка тем же владельцем снимается одним release', () => {
    setBodyScrollLock(first, true);
    setBodyScrollLock(first, true);
    releaseBodyScrollLock(first);
    expect(document.body.style.overflow).toBe('');
    expect(document.documentElement.style.overflow).toBe('');
  });

  it('release незнакомого владельца не трогает чужую блокировку', () => {
    setBodyScrollLock(first, true);
    releaseBodyScrollLock({});
    expect(document.body.style.overflow).toBe('hidden');
    expect(document.documentElement.style.overflow).toBe('hidden');
  });
});
