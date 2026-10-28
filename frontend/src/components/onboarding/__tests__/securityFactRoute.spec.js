import { describe, it, expect, beforeEach, vi } from 'vitest';

/**
 * Резолв фактовой таблицы для тура охраны на границе с сетью и правами.
 * Чистый отбор проверяется на resolveFactTableRoute (securityOnboardingSteps.spec.js),
 * здесь - проводка: что уходит в запрос, чем отсеивается недоступная таблица и что
 * происходит, пока права не загружены.
 */

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/stores/permissions', () => ({ usePermissionsStore: () => permissions }));

import { apiRequest } from '@/api/client';
import { getSecurityFactRoute, getSecurityPostContext } from '@/api/onboarding';

// Ссылка на объект берётся внутри фабрики лениво (вызов происходит уже в тесте),
// поэтому hoisting vi.mock ей не мешает.
const permissions = { loaded: true, hasPermission: vi.fn(() => true) };

const table = (name, type) => ({
  table: { name, table_type: type, show_fact_table: true, is_active: true },
});

function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) };
}

describe('getSecurityFactRoute', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    permissions.loaded = true;
    permissions.hasPermission = vi.fn(() => true);
  });

  it('берёт список системных таблиц и отдаёт машинную фактовую', async () => {
    apiRequest.mockResolvedValue(okJson([table('people_1', 'people'), table('kpp_1', 'cars')]));

    await expect(getSecurityFactRoute()).resolves.toBe('/table/kpp_1');
    expect(apiRequest).toHaveBeenCalledWith('/system-tables');
  });

  it('таблица без права table.<name>.view пропускается - тур не поведёт на роут-гард', async () => {
    apiRequest.mockResolvedValue(okJson([table('kpp_1', 'cars'), table('people_1', 'people')]));
    permissions.hasPermission = vi.fn((key) => key !== 'table.kpp_1.view');

    await expect(getSecurityFactRoute()).resolves.toBe('/table/people_1');
    expect(permissions.hasPermission).toHaveBeenCalledWith('table.kpp_1.view');
  });

  it('ни одной доступной таблицы - null', async () => {
    apiRequest.mockResolvedValue(okJson([table('kpp_1', 'cars')]));
    permissions.hasPermission = vi.fn(() => false);

    await expect(getSecurityFactRoute()).resolves.toBe(null);
  });

  it('права ещё не загружены - не делает запрос и не выбирает пост без проверки', async () => {
    apiRequest.mockResolvedValue(okJson([table('kpp_1', 'cars')]));
    permissions.loaded = false;
    permissions.hasPermission = vi.fn(() => false);

    await expect(getSecurityFactRoute()).rejects.toThrow('not ready');
    expect(apiRequest).not.toHaveBeenCalled();
    expect(permissions.hasPermission).not.toHaveBeenCalled();
  });

  it('ошибки ответа и сети не выдают известное отсутствие поста', async () => {
    apiRequest.mockResolvedValue({ ok: false, status: 500, json: vi.fn().mockResolvedValue({}) });
    await expect(getSecurityFactRoute()).rejects.toThrow();

    apiRequest.mockRejectedValue(new Error('network'));
    await expect(getSecurityFactRoute()).rejects.toThrow('network');
  });

  it('сохраняет обычный пост и его metadata, когда фактовая таблица выключена', async () => {
    apiRequest.mockResolvedValue(okJson([{ table: { name: 'post', table_type: 'cars', show_fact_table: false, is_active: true } }]));
    await expect(getSecurityPostContext()).resolves.toEqual({ route: '/table/post', tableType: 'cars', showFactTable: false });
  });
  it('не принимает повреждённый список за пустой', async () => {
    apiRequest.mockResolvedValue(okJson({}));
    await expect(getSecurityPostContext()).rejects.toThrow('Unknown learning tables');
  });
});
