import { describe, it, expect, vi, beforeEach } from 'vitest';
import { apiRequest } from '@/api/client';
import { fetchCurrentUserName } from '../currentUserName';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

/**
 * Подпись «Отчёт сформировал» в выгрузках справочников. Имя необязательное: сбой
 * запроса не должен ронять экран, поэтому наружу уходит пустая строка, а не ошибка.
 */
describe('fetchCurrentUserName', () => {
  beforeEach(() => vi.clearAllMocks());

  const reply = (body, ok = true) => apiRequest.mockResolvedValue({ ok, json: vi.fn().mockResolvedValue(body) });

  it('собирает ФИО из частей', async () => {
    reply({ last_name: 'Иванов', first_name: 'Иван', middle_name: 'Иванович', username: 'ivanov' });
    await expect(fetchCurrentUserName()).resolves.toBe('Иванов Иван Иванович');
  });

  it('пропускает пустые части', async () => {
    reply({ last_name: 'Иванов', first_name: '', middle_name: null, username: 'ivanov' });
    await expect(fetchCurrentUserName()).resolves.toBe('Иванов');
  });

  it('без ФИО подставляет логин', async () => {
    reply({ username: 'ivanov' });
    await expect(fetchCurrentUserName()).resolves.toBe('ivanov');
  });

  it('отказ сервера отдаёт пустую строку, а не бросает', async () => {
    reply({}, false);
    await expect(fetchCurrentUserName()).resolves.toBe('');
  });

  it('сетевая ошибка тоже гасится', async () => {
    apiRequest.mockRejectedValue(new Error('network'));
    await expect(fetchCurrentUserName()).resolves.toBe('');
  });
});
