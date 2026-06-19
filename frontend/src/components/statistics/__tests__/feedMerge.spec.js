import { describe, it, expect } from 'vitest';
import { mergeFeed, feedRowKey } from '../feedMerge.js';

const row = (created_at, subject, action_type = 'entry') => ({ created_at, subject, action_type });

describe('mergeFeed', () => {
  it('на первой загрузке возвращает входящие записи', () => {
    const incoming = [row('2026-06-19T10:00:00Z', 'Иванов'), row('2026-06-19T09:00:00Z', 'Петров')];
    expect(mergeFeed([], incoming)).toEqual(incoming);
  });

  it('новые записи добавляет сверху, существующие оставляет на месте', () => {
    const current = [row('2026-06-19T10:00:00Z', 'Иванов'), row('2026-06-19T09:00:00Z', 'Петров')];
    const incoming = [row('2026-06-19T11:00:00Z', 'Сидоров'), ...current];

    const merged = mergeFeed(current, incoming);

    expect(merged).toHaveLength(3);
    expect(merged[0].subject).toBe('Сидоров');
    // старые строки — те же объекты (Vue не перемонтирует их по стабильному ключу)
    expect(merged[1]).toBe(current[0]);
    expect(merged[2]).toBe(current[1]);
  });

  it('при отсутствии новых возвращает тот же массив (ссылочная стабильность)', () => {
    const current = [row('2026-06-19T10:00:00Z', 'Иванов')];
    const incoming = [row('2026-06-19T10:00:00Z', 'Иванов')];
    expect(mergeFeed(current, incoming)).toBe(current);
  });

  it('различает запись по направлению при одинаковом времени и субъекте', () => {
    const current = [row('2026-06-19T10:00:00Z', 'Иванов', 'entry')];
    const incoming = [row('2026-06-19T10:00:00Z', 'Иванов', 'exit'), ...current];

    const merged = mergeFeed(current, incoming);

    expect(merged).toHaveLength(2);
    expect(merged[0].action_type).toBe('exit');
  });

  it('ограничивает размер ленты параметром max', () => {
    const current = Array.from({ length: 50 }, (_, i) => row(`t${i}`, `s${i}`));
    const incoming = [row('new', 'newcomer'), ...current];

    const merged = mergeFeed(current, incoming, 50);

    expect(merged).toHaveLength(50);
    expect(merged[0].subject).toBe('newcomer');
  });

  it('feedRowKey собирает ключ из времени, субъекта и направления', () => {
    expect(feedRowKey(row('2026-06-19T10:00:00Z', 'Иванов', 'entry')))
      .toBe('2026-06-19T10:00:00Z|Иванов|entry');
  });
});
