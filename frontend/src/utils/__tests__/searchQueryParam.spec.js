import { describe, it, expect, vi } from 'vitest';
import { readSearchFromRoute, writeSearchToRoute, QUERY_PARAM } from '../searchQueryParam';

// Связка строки поиска с адресом ломается в двух местах: затиранием соседних
// параметров (рядом живут признак архива и фильтры) и необработанной отменой
// навигации при быстром вводе. Оба случая закрыты тестами.

function makeRouter() {
  return { replace: vi.fn().mockResolvedValue(undefined) };
}

describe('readSearchFromRoute', () => {
  it('читает канонический параметр', () => {
    expect(readSearchFromRoute({ query: { q: 'Роголев' } })).toBe('Роголев');
  });

  it('читает историческое имя как синоним', () => {
    expect(readSearchFromRoute({ query: { search: 'Роголев' } })).toBe('Роголев');
  });

  it('канонический параметр важнее синонима', () => {
    expect(readSearchFromRoute({ query: { q: 'новый', search: 'старый' } })).toBe('новый');
  });

  it('пустой адрес даёт пустую строку, а не undefined', () => {
    expect(readSearchFromRoute({ query: {} })).toBe('');
    expect(readSearchFromRoute(undefined)).toBe('');
  });

  it('массив в параметре игнорируется', () => {
    expect(readSearchFromRoute({ query: { q: ['a', 'b'] } })).toBe('');
  });
});

describe('writeSearchToRoute', () => {
  it('сохраняет соседние параметры', () => {
    const router = makeRouter();
    const route = { query: { archive: 'true', type: 'cars' } };

    writeSearchToRoute(router, route, 'Роголев');

    expect(router.replace).toHaveBeenCalledWith({
      query: { archive: 'true', type: 'cars', [QUERY_PARAM]: 'Роголев' },
    });
  });

  it('пустая строка убирает параметр, не оставляя ?q=', () => {
    const router = makeRouter();
    const route = { query: { q: 'Роголев', archive: 'true' } };

    writeSearchToRoute(router, route, '   ');

    expect(router.replace).toHaveBeenCalledWith({ query: { archive: 'true' } });
  });

  it('историческое имя вычищается при первой же записи', () => {
    const router = makeRouter();
    const route = { query: { search: 'старый' } };

    writeSearchToRoute(router, route, 'новый');

    expect(router.replace).toHaveBeenCalledWith({ query: { [QUERY_PARAM]: 'новый' } });
  });

  it('без изменений навигация не вызывается', () => {
    const router = makeRouter();
    const route = { query: { q: 'Роголев' } };

    writeSearchToRoute(router, route, 'Роголев');

    expect(router.replace).not.toHaveBeenCalled();
  });

  it('отменённая навигация не роняет страницу', async () => {
    const router = { replace: vi.fn().mockRejectedValue(new Error('Navigation aborted')) };

    expect(() => writeSearchToRoute(router, { query: {} }, 'Роголев')).not.toThrow();
    await Promise.resolve();
  });
});
